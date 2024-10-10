package handler

import (
	"fmt"
    "time"
    "math/big"
	"crypto/rand"
	"encoding/base64"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func generateRandomString(n int) (string, error) {
    b := make([]byte, n)
    _, err := rand.Read(b)
    if err != nil {
        return "", err
    }

    // Encode to base64 and trim to the desired length
    return base64.URLEncoding.EncodeToString(b)[:n], nil
}

func generateUnique15DigitInt() (int, error) {
    // The maximum value for a 15-digit number
    max := new(big.Int)
    max.SetString("999999999999999", 10) // 15-digit maximum value

	// Get *big.Int type of num.
    bigNum, err := rand.Int(rand.Reader, max)
    if err != nil {
        return 0, err
    }

    result, err := bigIntToInt(bigNum)
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("Converted int:", result)
    }

    return result, nil
}

func bigIntToInt(b *big.Int) (int, error) {
    // Convert big.Int to int64
    if !b.IsInt64() {
        return 0, fmt.Errorf("value out of int64 range")
    }
    // Ensure it's within int range
    i := b.Int64()
    if i > int64(^uint(0)>>1) || i < int64(^int(0)) {
        return 0, fmt.Errorf("value out of int range")
    }
    return int(i), nil
}

func generateUnique15DigitString() (int, error) {
    // The maximum value for a 15-digit number
    max := new(big.Int)
    max.SetString("999999999999999", 10) // 15-digit maximum value

	// Get *big.Int type of num.
    bigNum, err := rand.Int(rand.Reader, max)
    if err != nil {
        return 0, err
    }

    result, err := bigIntToInt(bigNum)
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Println("Converted int:", result)
    }

    return result, nil
}

func getSeoulCurrentTime() string {
	loc, _ := time.LoadLocation("Asia/Seoul")
	currentTime := time.Now().In(loc)	
	return currentTime.Format("2006-01-02 15:04:05")
}
