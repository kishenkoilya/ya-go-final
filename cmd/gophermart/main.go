package main

import (
	"go.uber.org/zap"
)

var sugar zap.SugaredLogger

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	sugar = *logger.Sugar()

	config := getVars()
	config.printConfig()
	runServer(config)
}

// cd "C:\Program Files\PostgreSQL\12\bin"
// ./psql -d postgres -U postgres
// drop table GophermartUsers, gophermartorders, gophermartauthentications;
// go run ./cmd\gophermart\ -a localhost:8080 -d postgresql://postgres:gpadmin@localhost:5432/postgres?sslmode=disable
// curl -i -X POST -H "Content-Type: application/json" -d "{\"login\": \"megalogin\", \"password\": \"pass1\"}" http://localhost:8080/api/user/register
// curl -i -X POST -H "Content-Type: application/json" -d "{\"login\": \"megalogin\", \"password\": \"pass1\"}" http://localhost:8080/api/user/login
// curl -X POST "http://localhost:8080/api/user/orders" -H "Authorization: $argon2id$v=19$m=65536,t=1,p=4$iRGqQXe4YVHqGuOSPbqRU0Y6HrHvDvxCndleeQpZUxCofusEFsiVY/e91ycjqmKx -H "Content-Type: text/plain" --data "4561261212345467"
// curl -X GET "http://localhost:8080/api/user/balance" -H "Authorization: $argon2id$v=19$m=65536,t=1,p=4$iRGqQXe4YVHqGuOSPbqRU0Y6HrHvDvxCndleeQpZUxCofusEFsiVY/e91ycjqmKx
// 4561261212345464
// 12345678903
