package main

import (
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/pgtype"
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

func NewDBConnection(databaseUri string) RetryFunc {
	return func() (interface{}, error) {
		connConfig, err := pgx.ParseConnectionString(databaseUri)
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
		query := `CREATE TABLE IF NOT EXISTS GophermartUsers (
			id SERIAL PRIMARY KEY, 
			login VARCHAR(250) UNIQUE, 
			salt BYTEA NOT NULL, 
			password_hash TEXT, 
			current_balance DOUBLE PRECISION, 
			balance_withdrawn DOUBLE PRECISION);`
		res, err := db.conn.Exec(query)
		if err != nil {
			return nil, err
		}
		sugar.Infoln(res)

		query = `CREATE TABLE IF NOT EXISTS GophermartAuthentications (
			id SERIAL PRIMARY KEY, 
			login_id INTEGER REFERENCES GophermartUsers(id) NOT NULL, 
			token TEXT NOT NULL, 
			active BOOLEAN);`
		res, err = db.conn.Exec(query)
		if err != nil {
			return nil, err
		}
		sugar.Infoln(res)

		query = `CREATE TABLE IF NOT EXISTS GophermartOrders (
			id SERIAL PRIMARY KEY, 
			login_id INTEGER REFERENCES GophermartUsers(id) NOT NULL, 
			number VARCHAR(50) UNIQUE, 
			status VARCHAR(50) NOT NULL, 
			accrual DOUBLE PRECISION, 
			withdrawn DOUBLE PRECISION, 
			uploaded_at TIMESTAMPTZ NOT NULL);`
		res, err = db.conn.Exec(query)
		if err != nil {
			return nil, err
		}
		sugar.Infoln(res)

		return nil, nil
	}
}

func (db *DBConnection) WriteNewUserInfo(login, hash, salt string) RetryFunc {
	return func() (interface{}, error) {
		query := `INSERT INTO GophermartUsers 
		(login, salt, password_hash, current_balance, balance_withdrawn) 
		VALUES($1, $2, $3, 0, 0)`
		res, err := db.conn.Exec(query, login, hash, salt)
		if err != nil {
			return nil, err
		}
		sugar.Infoln(res)

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
		query := `SELECT salt, password_hash 
		FROM GophermartUsers 
		WHERE login=$1`
		res, err := db.conn.Query(query, login)
		if err != nil {
			return nil, err
		}
		uInfo := UserInfo{Login: login}
		for res.Next() {
			err := res.Scan(&uInfo.Salt, &uInfo.Hash)
			if err != nil {
				return nil, err
			}
		}
		return &uInfo, nil
	}
}

func (db *DBConnection) CreateAuthToken(login, hash string) RetryFunc {
	return func() (interface{}, error) {
		query := `SELECT COUNT(*) FROM GophermartAuthentications`
		var authNum int
		err := db.conn.QueryRow(query).Scan(&authNum)
		if err != nil {
			return nil, err
		}
		query = `SELECT id FROM GophermartUsers WHERE login=$1`
		var loginID int
		err = db.conn.QueryRow(query, login).Scan(&loginID)
		if err != nil {
			return nil, err
		}

		token := HashBase64(fmt.Sprint(authNum), hash, fmt.Sprint(loginID+1))

		query = `INSERT INTO GophermartAuthentications 
		(login_id, token) 
		VALUES($1, $2)`
		res, err := db.conn.Exec(query, loginID, token)
		if err != nil {
			return nil, err
		}
		sugar.Infoln(res)
		return token, nil
	}
}

func (db *DBConnection) CheckAuthToken(auth string) RetryFunc {
	return func() (interface{}, error) {
		query := `SELECT login_id 
		FROM GophermartAuthentications 
		WHERE token=$1 AND active=TRUE`
		var loginID int
		err := db.conn.QueryRow(query, auth).Scan(&loginID)
		if err != nil {
			return -1, err
		}
		return loginID, nil
	}
}

func (db *DBConnection) LoadOrderNumber(loginID int, orderNum string) RetryFunc {
	return func() (interface{}, error) {
		query := `INSERT INTO GophermartOrders 
		(login_id, number, status, accrual, withdrawn, uploaded_at) 
		VALUES($1, $2, "NEW", 0, 0, $3)`
		res, err := db.conn.Exec(query, loginID, orderNum, time.Now().UTC())
		if err != nil {
			return -1, err
		}
		sugar.Infoln(res)
		var orderID int
		query = `SELECT id FROM GophermartOrders WHERE number=$1`
		err = db.conn.QueryRow(query, orderNum).Scan(&orderID)
		if err != nil {
			return -1, err
		}
		return orderID, nil
	}
}

func (db *DBConnection) UpdateOrder(loginID int, accrual float64, orderNum, status string) RetryFunc {
	return func() (interface{}, error) {
		query := `UPDATE GophermartOrders 
		SET accrual=$1, status=$2
		WHERE number=$3`
		res, err := db.conn.Exec(query, loginID, status, orderNum)
		if err != nil {
			return nil, err
		}
		sugar.Infoln(res)
		return nil, nil
	}
}

func (db *DBConnection) AddLoyaltyPoints(loginID int, accrual float64) RetryFunc {
	return func() (interface{}, error) {
		query := `UPDATE GophermartUsers 
		SET current_balance=current_balance+$1
		WHERE id=$2`
		res, err := db.conn.Exec(query, accrual, loginID)
		if err != nil {
			return nil, err
		}
		sugar.Infoln(res)
		return nil, nil
	}
}

type OrderInfo struct {
	Number      string `json:"number"`
	Status      string `json:"status"`
	Accrual     string `json:"accrual"`
	Uploaded_at string `json:"uploaded_at"`
}

func (db *DBConnection) GetOrdersInfo(loginID int) RetryFunc {
	return func() (interface{}, error) {
		var orders []OrderInfo
		query := `SELECT number, status, accrual, uploaded_at 
		FROM GophermartOrders 
		WHERE login_id=$1`
		res, err := db.conn.Query(query, loginID)
		if err != nil {
			return nil, err
		}
		for res.Next() {
			var order OrderInfo
			var myTime pgtype.Timestamptz
			err := res.Scan(&order.Number, &order.Status, &order.Accrual, &myTime)
			if err != nil {
				return nil, err
			}
			order.Uploaded_at = myTime.Time.Format(time.RFC3339)
			orders = append(orders, order)
		}

		return &orders, nil
	}
}

type BalanceInfo struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

func (db *DBConnection) GetBalanceInfo(loginID int) RetryFunc {
	return func() (interface{}, error) {
		query := `SELECT current_balance, balance_withdrawn 
		FROM GophermartUsers 
		WHERE login_id=$1`
		res, err := db.conn.Query(query, loginID)
		if err != nil {
			return nil, err
		}
		bInfo := BalanceInfo{}
		for res.Next() {
			err := res.Scan(&bInfo.Current, &bInfo.Withdrawn)
			if err != nil {
				return nil, err
			}
		}
		return &bInfo, nil
	}
}

func (db *DBConnection) WithdrawBalance(loginID int, order string, sum float64) RetryFunc {
	return func() (interface{}, error) {
		query := `SELECT current_balance, balance_withdrawn 
		FROM GophermartUsers 
		WHERE login_id=$1`
		res, err := db.conn.Query(query, loginID)
		if err != nil {
			return nil, err
		}
		bInfo := BalanceInfo{}
		for res.Next() {
			err := res.Scan(&bInfo.Current, &bInfo.Withdrawn)
			if err != nil {
				return nil, err
			}
		}
		if bInfo.Current < sum {
			return false, nil
		}
		query = `INSERT INTO GophermartOrders 
		(login_id, number, status, accrual, withdrawn, uploaded_at) 
		VALUES($1, $2, "NEW", 0, $3, $4)`
		res2, err := db.conn.Exec(query, loginID, order, sum, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		sugar.Infoln(res2)

		query = `UPDATE GophermartUsers 
		SET current_balance=$1, balance_withdrawn=$2
		WHERE id=$3`
		res3, err := db.conn.Exec(query, bInfo.Current-sum, bInfo.Withdrawn+sum, loginID)
		if err != nil {
			return nil, err
		}
		sugar.Infoln(res3)
		return true, nil
	}
}

type WithdrawalsInfo struct {
	Order       string  `json:"order"`
	Sum         float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
}

func (db *DBConnection) GetWithdrawalsInfo(loginID int) RetryFunc {
	return func() (interface{}, error) {
		var withdrawals []WithdrawalsInfo
		query := `SELECT number, withdrawn, uploaded_at 
		FROM GophermartOrders
		WHERE login_id=$1`
		res, err := db.conn.Query(query, loginID)
		if err != nil {
			return nil, err
		}
		for res.Next() {
			var withdrawal WithdrawalsInfo
			var myTime pgtype.Timestamptz
			err := res.Scan(&withdrawal.Order, &withdrawal.Sum, &myTime)
			if err != nil {
				return nil, err
			}
			withdrawal.ProcessedAt = myTime.Time.Format(time.RFC3339)
			withdrawals = append(withdrawals, withdrawal)
		}

		return &withdrawals, nil
	}
}
