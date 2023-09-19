package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/julienschmidt/httprouter"
)

// POST /api/user/register — регистрация пользователя;
// POST /api/user/login — аутентификация пользователя;
// POST /api/user/orders — загрузка пользователем номера заказа для расчёта;
// GET /api/user/orders — получение списка загруженных пользователем номеров заказов, статусов их обработки и информации о начислениях;
// GET /api/user/balance — получение текущего баланса счёта баллов лояльности пользователя;
// POST /api/user/balance/withdraw — запрос на списание баллов с накопительного счёта в счёт оплаты нового заказа;
// GET /api/user/withdrawals — получение информации о выводе средств с накопительного счёта пользователем.

func runServer(config *Config) {
	var handlerVars *HandlerVars

	router := httprouter.New()
	router.POST("/api/user/register", LoggingMiddleware(GzipMiddleware(ParamsMiddleware(registerPage, handlerVars))))
	router.POST("/api/user/login", LoggingMiddleware(GzipMiddleware(ParamsMiddleware(loginPage, handlerVars))))
	router.POST("/api/user/orders", LoggingMiddleware(GzipMiddleware(ParamsMiddleware(postOrdersPage, handlerVars))))
	router.GET("/api/user/orders", LoggingMiddleware(GzipMiddleware(ParamsMiddleware(getOrdersPage, handlerVars))))
	router.GET("/api/user/balance", LoggingMiddleware(GzipMiddleware(ParamsMiddleware(balancePage, handlerVars))))
	router.POST("/api/user/balance/withdraw", LoggingMiddleware(GzipMiddleware(ParamsMiddleware(balanceWithdrawPage, handlerVars))))
	router.GET("/api/user/withdrawals", LoggingMiddleware(GzipMiddleware(ParamsMiddleware(withdrawalsPage, handlerVars))))

	server := &http.Server{
		Addr:    (*config).Address,
		Handler: router,
	}
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			sugar.Fatalw(err.Error(), "event", "start server")
		}
	}()
}

type LoginInfo struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func registerPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "Request content type is not json!", http.StatusBadRequest)
		return
	}

	var loginInfo LoginInfo
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Could not read request body!", http.StatusInternalServerError)
		return
	}
	err = json.Unmarshal(bodyBytes, &loginInfo)
	if err != nil {
		http.Error(w, "Could not unmarshal login info!", http.StatusBadRequest)
		return
	}

	hash, salt, err := HashPassword(loginInfo.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dbWriteNewUserInfoFunc := handlerVars.db.WriteNewUserInfo(loginInfo.Login, hash, salt)
	_, err = Retrypg(pgerrcode.ConnectionException, dbWriteNewUserInfoFunc)
	if err != nil {
		if err.Error() == "Login exists" {
			http.Error(w, "Login name already exists. Enter another.", http.StatusConflict)
			return
		}
		http.Error(w, "Connection to database error", http.StatusInternalServerError)
		return
	}

	dbCreateTokenFunc := handlerVars.db.CreateAuthToken(loginInfo.Login, hash)
	obj, err := Retrypg(pgerrcode.ConnectionException, dbCreateTokenFunc)
	token := obj.(string)
	w.Header().Set("Authorization", token)
	w.WriteHeader(http.StatusOK)
}

func loginPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "Request content type is not json!", http.StatusBadRequest)
		return
	}

	var loginInfo LoginInfo
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Could not read request body!", http.StatusInternalServerError)
		return
	}
	err = json.Unmarshal(bodyBytes, &loginInfo)
	if err != nil {
		http.Error(w, "Could not unmarshal login info!", http.StatusBadRequest)
		return
	}

	dbGetUserInfoFunc := handlerVars.db.GetUserInfo(loginInfo.Login)
	obj, err := Retrypg(pgerrcode.ConnectionException, dbGetUserInfoFunc)
	if err != nil {
		if err.Error() == "Login does not exist" {
			http.Error(w, "Login does not exist. Wrong login", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Connection to database error", http.StatusInternalServerError)
		return
	}
	userInfo := obj.(*UserInfo)

	check, err := CheckPassword(loginInfo.Password, userInfo.Salt, userInfo.Hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !check {
		http.Error(w, "Wrong password", http.StatusUnauthorized)
		return
	}

	dbCreateTokenFunc := handlerVars.db.CreateAuthToken(loginInfo.Login, userInfo.Hash)
	obj, err = Retrypg(pgerrcode.ConnectionException, dbCreateTokenFunc)
	token := obj.(string)
	w.Header().Set("Authorization", token)
	w.WriteHeader(http.StatusOK)
}

func authorization(authData string, db *DBConnection) (int, error) {
	if authData == "" {
		err := errors.New("Unauthorized")
		return http.StatusUnauthorized, err
	}
	obj, err := Retrypg(pgerrcode.ConnectionException, db.CheckAuthToken(authData))
	if err != nil {
		return http.StatusInternalServerError, err
	}
	authorized := obj.(bool)
	if !authorized {
		err := errors.New("Unauthorized")
		return http.StatusUnauthorized, err
	}
	return http.StatusOK, nil
}

func uploadOrderNumber(auth, numb string, db *DBConnection) (int, error) {
	obj, err := Retrypg(pgerrcode.ConnectionException, db.LoadOrderNumber(auth, numb))
	if err != nil {
		return http.StatusInternalServerError, err
	}
	newOrderKey := obj.(int)
	if newOrderKey == -1 {
		err := errors.New("Order number already loaded.")
		return http.StatusConflict, err
	}
	return http.StatusAccepted, nil
}

func postOrdersPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	auth := r.Header.Get("Authorization")
	code, err := authorization(auth, handlerVars.db)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}

	if !strings.Contains(r.Header.Get("Content-Type"), "text/plain") {
		http.Error(w, "Request content type is not plain text!", http.StatusBadRequest)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Could not read request body!", http.StatusInternalServerError)
		return
	}
	orderNum := string(bodyBytes)
	c, err := CheckLuhn(orderNum)
	if err != nil {
		http.Error(w, "Luhn check could not be complete. "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !c {
		http.Error(w, "Incorrect order number format.", http.StatusUnprocessableEntity)
		return
	}

	code, err = uploadOrderNumber(auth, orderNum, handlerVars.db)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	w.WriteHeader(code)
}

func getOrdersPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	auth := r.Header.Get("Authorization")
	code, err := authorization(auth, handlerVars.db)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}

	obj, err := Retrypg(pgerrcode.ConnectionException, handlerVars.db.GetOrdersInfo(auth))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	orderInfo := obj.(*[]OrderInfo)
	if len(*orderInfo) == 0 {
		http.Error(w, "No order info.", http.StatusNoContent)
		return
	}

	respJSON, err := json.Marshal(&orderInfo)
	if err != nil {
		http.Error(w, "Response from database coult not be marshaled to json. "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respJSON)
}

func balancePage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	auth := r.Header.Get("Authorization")
	code, err := authorization(auth, handlerVars.db)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}

	obj, err := Retrypg(pgerrcode.ConnectionException, handlerVars.db.GetBalanceInfo(auth))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	balanceInfo := obj.(*BalanceInfo)

	respJSON, err := json.Marshal(&balanceInfo)
	if err != nil {
		http.Error(w, "Response from database coult not be marshaled to json. "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respJSON)
}

type WithdrawInfo struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

func withdrawBalance(auth, order string, sum float64, db *DBConnection) (int, error) {
	obj, err := Retrypg(pgerrcode.ConnectionException, db.WithdrawBalance(auth, order, sum))
	if err != nil {
		return http.StatusInternalServerError, err
	}
	success := obj.(bool)
	if !success {
		err := errors.New("Not enough balance.")
		return http.StatusPaymentRequired, err
	}
	return http.StatusOK, nil
}

func balanceWithdrawPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "Request content type is not json!", http.StatusBadRequest)
		return
	}

	auth := r.Header.Get("Authorization")
	code, err := authorization(auth, handlerVars.db)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}

	var withdrawInfo *WithdrawInfo
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Could not read request body!", http.StatusInternalServerError)
		return
	}
	err = json.Unmarshal(bodyBytes, &withdrawInfo)
	if err != nil {
		http.Error(w, "Could not unmarshal withdraw info!", http.StatusBadRequest)
		return
	}

	c, err := CheckLuhn(withdrawInfo.Order)
	if err != nil {
		http.Error(w, "Luhn check could not be complete. "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !c {
		http.Error(w, "Incorrect order number format.", http.StatusUnprocessableEntity)
		return
	}

	code, err = withdrawBalance(auth, withdrawInfo.Order, withdrawInfo.Sum, handlerVars.db)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	w.WriteHeader(code)
}

func withdrawalsPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	sugar.Infoln(handlerVars.psqlConnectLine)
}
