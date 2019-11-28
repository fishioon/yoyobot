package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-redis/redis"
)

const (
	rdsBalance = "yoyo:balance"
)

func DayAdd(db *redis.Client, uid string, adds int64) (balance int64, err error) {
	if balance, err = DayGet(db, uid); err != nil {
		return
	}
	if balance+adds < 0 {
		return 0, ErrInsufficientBalance
	}
	if balance, err = db.HIncrBy(rdsBalance, uid, adds).Result(); err != nil {
		return 0, ErrSystemError
	}
	if balance < 0 {
		if err = db.HIncrBy(rdsBalance, uid, -adds).Err(); err != nil {
			return 0, ErrSystemError
		}
		return 0, ErrInsufficientBalance
	}
	return balance, nil
}

func DayShow(db *redis.Client) (map[string]string, error) {
	parse := func(v string) string {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return "0"
		}
		return FormatAmount(n)
	}
	res, err := db.HGetAll(rdsBalance).Result()
	if err != nil {
		return nil, err
	}
	for k, v := range res {
		res[k] = parse(v)
	}
	return res, nil
}

func DayGet(db *redis.Client, uid string) (int64, error) {
	res, err := db.HGet(rdsBalance, uid).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, ErrSystemError
	}
	return strconv.ParseInt(res, 10, 64)
}

func DaySet(db *redis.Client, req map[string]int64) error {
	balance := make(map[string]interface{})
	for k, v := range req {
		balance[k] = v
	}
	return db.HMSet(rdsBalance, balance).Err()
}

func ParseAmount(s string) (int64, error) {
	s = strings.Replace(s, "天", "", -1)
	s = strings.TrimSpace(s)
	var adds int64
	if count := strings.Count(s, "."); count > 1 {
		return 0, ErrInvalidDays
	} else if count == 1 {
		if strings.HasSuffix(s, ".5") {
			s = strings.TrimRight(s, ".5")
			adds = 1
			if s[0] == '-' {
				adds = -adds
			}
		} else if strings.HasSuffix(s, ".0") {
			s = strings.TrimRight(s, ".0")
		} else {
			return 0, ErrInvalidDays
		}
	}
	if n, err := strconv.ParseInt(s, 10, 64); err != nil {
		return 0, ErrInvalidDays
	} else {
		adds += n * 2
	}
	return adds, nil
}

func FormatAmount(amount int64) string {
	var prefix, suffix string
	if amount < 0 {
		prefix = "-"
		amount = -amount
	}
	if amount%2 != 0 {
		suffix = ".5"
	}
	return fmt.Sprintf("%s%d%s", prefix, amount/2, suffix)
}
