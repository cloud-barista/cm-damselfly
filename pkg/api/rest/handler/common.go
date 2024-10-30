package handler

import (
	"fmt"
    "time"
    "math/big"
	"crypto/rand"
	"encoding/base64"
    "bufio"
	"os"
	"strings"
    "github.com/rs/zerolog/log"
    "path/filepath"
    "net"
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
	loc, err := time.LoadLocation("Asia/Seoul")
    if err != nil {
        log.Error().Msgf("Failed to Get the Time Value of the Location : [%v]", err)	
    }

	currentTime := time.Now().In(loc)	
	return currentTime.Format("2006-01-02 15:04:05")
}

func getModuleVersion(moduleName string) (string, error) {
    var goFile *os.File
    if isRunningInContainer() {
        wd, err := os.Getwd()
        if err != nil {
            return "", err
        }        
        goModPath := filepath.Join(wd, "go.mod")

        log.Debug().Msgf("go.mod file path : [%s]", goModPath)	

        var openErr error
        goFile, openErr = os.Open(goModPath)
        if openErr != nil {
            return "", openErr
        }
        defer goFile.Close()
    } else {
        var openErr error
        goFile, openErr = os.Open("./../../go.mod")
        if openErr != nil {
            return "", openErr
        }
        defer goFile.Close()
    }	

	scanner := bufio.NewScanner(goFile)
	for scanner.Scan() {
		line := scanner.Text()
        // log.Debug().Msgf("go.mod line : [%s]", line)	

		if strings.Contains(line, moduleName) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1], nil
			}
		}
	}
	return "", fmt.Errorf("Module [%s] not found", moduleName)
}

func isRunningInContainer() bool {
    interfaces, _ := net.Interfaces()
    for _, iface := range interfaces {
        // log.Debug().Msgf("iface.Name: [%v]", iface.Name)
        if strings.HasPrefix(iface.Name, "docker") {
            return true
        }
    }
    return false

    // file, err := os.Open("/proc/1/cgroup")
    // if err != nil {
    //     return false
    // }
    // defer file.Close()

    // scanner := bufio.NewScanner(file)
    // for scanner.Scan() {
    //     if strings.Contains(scanner.Text(), "docker") || strings.Contains(scanner.Text(), "kubepods") {
    //         return true
    //     }
    // }
    // return false
}
