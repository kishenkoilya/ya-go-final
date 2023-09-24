package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-resty/resty/v2"
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
	obj, err := Retrypg(pgerrcode.ConnectionException, NewDBConnection(config.DatabaseURI))
	if err != nil {
		panic(err)
	}
	handlerVars := &HandlerVars{AccrualSystemAddress: &config.AccrualSystemAddress}
	handlerVars.db = obj.(*DBConnection)
	dbCreateTokenFunc := handlerVars.db.InitTables()
	_, err = Retrypg(pgerrcode.ConnectionException, dbCreateTokenFunc)
	if err != nil {
		panic("Could not init tables. " + err.Error())
	}

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
	waitForShutdown(server, handlerVars)
	fmt.Println("Programm shutdown")
}

func waitForShutdown(server *http.Server, handlerVars *HandlerVars) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	<-signalChan
	fmt.Println("HTTP-server shutdown.")
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
		sugar.Errorln(err.Error())
		http.Error(w, "Could not read request body! "+err.Error(), http.StatusInternalServerError)
		return
	}
	err = json.Unmarshal(bodyBytes, &loginInfo)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, "Could not unmarshal login info! "+err.Error(), http.StatusBadRequest)
		return
	}

	hash, err := HashPassword(loginInfo.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dbWriteNewUserInfoFunc := handlerVars.db.WriteNewUserInfo(loginInfo.Login, hash)
	_, err = Retrypg(pgerrcode.ConnectionException, dbWriteNewUserInfoFunc)
	if err != nil {
		if err.Error() == "Login exists" {
			sugar.Errorln(err.Error())
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		sugar.Errorln(err.Error())
		http.Error(w, "Connection to database error", http.StatusInternalServerError)
		return
	}

	dbCreateTokenFunc := handlerVars.db.CreateAuthToken(loginInfo.Login, hash)
	obj, err := Retrypg(pgerrcode.ConnectionException, dbCreateTokenFunc)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, "Could not create authentication token. "+err.Error(), http.StatusInternalServerError)
	}
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
		sugar.Errorln(err.Error())
		http.Error(w, "Could not read request body! "+err.Error(), http.StatusInternalServerError)
		return
	}
	err = json.Unmarshal(bodyBytes, &loginInfo)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, "Could not unmarshal login info! "+err.Error(), http.StatusBadRequest)
		return
	}

	dbGetUserInfoFunc := handlerVars.db.GetUserInfo(loginInfo.Login)
	obj, err := Retrypg(pgerrcode.ConnectionException, dbGetUserInfoFunc)
	if err != nil {
		if err.Error() == "Login does not exist" {
			sugar.Errorln(err.Error())
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		sugar.Errorln(err.Error())
		http.Error(w, "Connection to database error", http.StatusInternalServerError)
		return
	}
	userInfo := obj.(*UserInfo)

	check := CheckPassword(loginInfo.Password, userInfo.Hash)
	if !check {
		http.Error(w, "Wrong password", http.StatusUnauthorized)
		return
	}

	dbCreateTokenFunc := handlerVars.db.CreateAuthToken(loginInfo.Login, userInfo.Hash)
	obj, err = Retrypg(pgerrcode.ConnectionException, dbCreateTokenFunc)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, "Could not create authentication token. "+err.Error(), http.StatusInternalServerError)
	}
	token := obj.(string)
	w.Header().Set("Authorization", token)
	w.WriteHeader(http.StatusOK)
}

func authorization(authData string, db *DBConnection) (int, int, error) {
	if authData == "" {
		err := errors.New("Unauthorized")
		return http.StatusUnauthorized, -1, err
	}
	obj, err := Retrypg(pgerrcode.ConnectionException, db.CheckAuthToken(authData))
	if err != nil {
		return http.StatusInternalServerError, -1, err
	}
	loginID := obj.(int)
	if loginID == -1 {
		err := errors.New("Unauthorized")
		return http.StatusUnauthorized, -1, err
	}
	return http.StatusOK, loginID, nil
}

func uploadOrderNumber(loginID int, numb string, db *DBConnection) (int, error) {
	obj, err := Retrypg(pgerrcode.ConnectionException, db.LoadOrderNumber(loginID, numb))
	if err != nil {
		sugar.Errorln(err.Error())
		return http.StatusInternalServerError, err
	}
	newOrderKey := obj.(int)
	if newOrderKey == -1 {
		err := errors.New("order number already loaded")
		return http.StatusConflict, err
	}
	return http.StatusAccepted, nil
}

type ASAAnswer struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual"`
}

func updateOrder(loginID int, numb string, handlerVars *HandlerVars) {
	client := resty.New()
	for {
		resp, err := client.R().Get(*handlerVars.AccrualSystemAddress + "/api/orders/" + numb)
		if err != nil {
			sugar.Errorln(err.Error())
		}
		var ans ASAAnswer
		err = json.Unmarshal(resp.Body(), &ans)
		if err != nil {
			sugar.Errorln(err.Error())
			time.Sleep(time.Second * 5)
			continue
		}
		sugar.Infoln(ans)

		_, err = Retrypg(pgerrcode.ConnectionException, handlerVars.db.UpdateOrder(loginID, ans.Accrual, ans.Order, ans.Status))
		if err != nil {
			panic(err)
		}

		if ans.Accrual > 0 {
			_, err = Retrypg(pgerrcode.ConnectionException, handlerVars.db.AddLoyaltyPoints(loginID, ans.Accrual))
			if err != nil {
				panic(err)
			}
		}

		if ans.Status == "INVALID" || ans.Status == "PROCESSED" {
			break
		}
	}
}

func postOrdersPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	auth := r.Header.Get("Authorization")
	code, loginID, err := authorization(auth, handlerVars.db)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, err.Error(), code)
		return
	}

	if !strings.Contains(r.Header.Get("Content-Type"), "text/plain") {
		http.Error(w, "Request content type is not plain text!", http.StatusBadRequest)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, "Could not read request body!", http.StatusInternalServerError)
		return
	}
	orderNum := string(bodyBytes)
	c, err := CheckLuhn(orderNum)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, "Luhn check could not be complete. "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !c {
		http.Error(w, "Incorrect order number format.", http.StatusUnprocessableEntity)
		return
	}

	code, err = uploadOrderNumber(loginID, orderNum, handlerVars.db)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, err.Error(), code)
		return
	}
	go updateOrder(loginID, orderNum, handlerVars)
	w.WriteHeader(code)
}

func getOrdersPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	auth := r.Header.Get("Authorization")
	code, loginID, err := authorization(auth, handlerVars.db)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, err.Error(), code)
		return
	}

	obj, err := Retrypg(pgerrcode.ConnectionException, handlerVars.db.GetOrdersInfo(loginID))
	if err != nil {
		sugar.Errorln(err.Error())
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
		sugar.Errorln(err.Error())
		http.Error(w, "Response from database coult not be marshaled to json. "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	sugar.Infoln(string(respJSON))
	w.Write(respJSON)
}

func balancePage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	auth := r.Header.Get("Authorization")
	code, loginID, err := authorization(auth, handlerVars.db)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, err.Error(), code)
		return
	}

	obj, err := Retrypg(pgerrcode.ConnectionException, handlerVars.db.GetBalanceInfo(loginID))
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	balanceInfo := obj.(*BalanceInfo)

	respJSON, err := json.Marshal(&balanceInfo)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, "Response from database coult not be marshaled to json. "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	sugar.Infoln(string(respJSON))
	w.Write(respJSON)
}

type WithdrawInfo struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

func withdrawBalance(loginID int, order string, sum float64, db *DBConnection) (int, error) {
	obj, err := Retrypg(pgerrcode.ConnectionException, db.WithdrawBalance(loginID, order, sum))
	if err != nil {
		sugar.Errorln(err.Error())
		return http.StatusInternalServerError, err
	}
	success := obj.(bool)
	if !success {
		err := errors.New("not enough balance")
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
	code, loginID, err := authorization(auth, handlerVars.db)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, err.Error(), code)
		return
	}

	var withdrawInfo *WithdrawInfo
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, "Could not read request body!", http.StatusInternalServerError)
		return
	}
	err = json.Unmarshal(bodyBytes, &withdrawInfo)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, "Could not unmarshal withdraw info!", http.StatusBadRequest)
		return
	}

	c, err := CheckLuhn(withdrawInfo.Order)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, "Luhn check could not be complete. "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !c {
		http.Error(w, "Incorrect order number format.", http.StatusUnprocessableEntity)
		return
	}

	code, err = withdrawBalance(loginID, withdrawInfo.Order, withdrawInfo.Sum, handlerVars.db)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, err.Error(), code)
		return
	}
	w.WriteHeader(code)
}

func withdrawalsPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	auth := r.Header.Get("Authorization")
	code, loginID, err := authorization(auth, handlerVars.db)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, err.Error(), code)
		return
	}

	obj, err := Retrypg(pgerrcode.ConnectionException, handlerVars.db.GetWithdrawalsInfo(loginID))
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	withdrawalsInfo := obj.(*WithdrawalsInfo)

	respJSON, err := json.Marshal(&withdrawalsInfo)
	if err != nil {
		sugar.Errorln(err.Error())
		http.Error(w, "Response from database coult not be marshaled to json. "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	sugar.Infoln(string(respJSON))
	w.Write(respJSON)
}
