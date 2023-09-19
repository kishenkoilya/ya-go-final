package main

import (
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx"
)

type RetryFunc func() (interface{}, error)

func Retrypg(errClass string, f RetryFunc) (interface{}, error) {
	var result interface{}
	var err error

	for i := 0; i < 3; i++ {
		result, err = f()
		if err == nil {
			return result, nil
		} else {
			if pgerr, ok := err.(pgx.PgError); ok {
				if errCodeCompare(errClass, pgerr.Code) {
					switch i {
					case 0:
						time.Sleep(1 * time.Second)
					case 1:
						time.Sleep(3 * time.Second)
					case 2:
						time.Sleep(5 * time.Second)
					default:
						return nil, err
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("all %d attempts failed: %w", 3, err)
}

func errCodeCompare(errClass, errCode string) bool {
	switch errClass {
	case pgerrcode.ConnectionException:
		return pgerrcode.IsConnectionException(errCode)
	case pgerrcode.OperatorIntervention:
		return pgerrcode.IsOperatorIntervention(errCode)
	default:
		return false
	}
}

type DBConnection struct {
	conn *pgx.Conn
}

func NewDBConnection(psqlLine string) RetryFunc {
	return func() (interface{}, error) {
		connConfig, err := pgx.ParseConnectionString(psqlLine)
		if err != nil {
			return nil, err
		}
		db, err := pgx.Connect(connConfig)
		if err != nil {
			return nil, err
		}
		return &DBConnection{db}, nil
	}
}

func (db *DBConnection) Close() error {
	return db.conn.Close()
}

func (db *DBConnection) InitTables() RetryFunc {
	return func() (interface{}, error) {
		query := `CREATE TABLE IF NOT EXISTS gauges (id SERIAL PRIMARY KEY, name VARCHAR(50), value double precision);`
		res, err := db.conn.Exec(query)
		if err != nil {
			return nil, err
		}
		fmt.Println(res)
		query = `CREATE TABLE IF NOT EXISTS counters (id SERIAL PRIMARY KEY, name VARCHAR(50), value bigint);`
		res, err = db.conn.Exec(query)
		if err != nil {
			return nil, err
		}
		fmt.Println(res)
		return nil, nil
	}
}

func (db *DBConnection) WriteNewUserInfo(login, hash, salt string) RetryFunc {
	return func() (interface{}, error) {
		return nil, nil
	}
}

type UserInfo struct {
	Login string
	Salt  string
	Hash  string
}

func (db *DBConnection) GetUserInfo(login string) RetryFunc {
	return func() (interface{}, error) {
		return nil, nil
	}
}

func (db *DBConnection) CreateAuthToken(login, hash string) RetryFunc {
	return func() (interface{}, error) {
		return nil, nil
	}
}

func (db *DBConnection) CheckAuthToken(auth string) RetryFunc {
	return func() (interface{}, error) {
		return nil, nil
	}
}

func (db *DBConnection) LoadOrderNumber(auth, orderNum string) RetryFunc {
	return func() (interface{}, error) {
		return nil, nil
	}
}

type OrderInfo struct {
	Number      string `json:"number"`
	Status      string `json:"status"`
	Accrual     string `json:"accrual"`
	Uploaded_at string `json:"uploaded_at"`
}

func (db *DBConnection) GetOrdersInfo(auth string) RetryFunc {
	return func() (interface{}, error) {
		return nil, nil
	}
}

type BalanceInfo struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

func (db *DBConnection) GetBalanceInfo(auth string) RetryFunc {
	return func() (interface{}, error) {
		return nil, nil
	}
}

func (db *DBConnection) WithdrawBalance(auth, order string, sum float64) RetryFunc {
	return func() (interface{}, error) {
		return nil, nil
	}
}
