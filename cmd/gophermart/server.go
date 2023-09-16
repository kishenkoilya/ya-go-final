package main

import (
	"encoding/json"
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
	requestBody := r.Body
	bodyBytes, err := io.ReadAll(requestBody)
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
	w.Header().Set("Authorization", hash) //TODO: Придумать токен авторизации
	w.WriteHeader(http.StatusOK)
}

func loginPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "Request content type is not json!", http.StatusBadRequest)
		return
	}

	var loginInfo LoginInfo
	requestBody := r.Body
	bodyBytes, err := io.ReadAll(requestBody)
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

	w.Header().Set("Authorization", userInfo.Hash) //TODO: Придумать токен авторизации
	w.WriteHeader(http.StatusOK)
}

func postOrdersPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	sugar.Infoln(handlerVars.psqlConnectLine)
}

func getOrdersPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	sugar.Infoln(handlerVars.psqlConnectLine)
}

func balancePage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	sugar.Infoln(handlerVars.psqlConnectLine)
}

func balanceWithdrawPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	sugar.Infoln(handlerVars.psqlConnectLine)
}

func withdrawalsPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handlerVars := r.Context().Value(HandlerVars{}).(*HandlerVars)
	sugar.Infoln(handlerVars.psqlConnectLine)
}
