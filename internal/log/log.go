package log

import (
	"log"
	"os"
)

var logger *log.Logger
var debugEnabled bool

func init() {
	file, err := os.OpenFile("sikmacode_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	logger = log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	debugEnabled = os.Getenv("SIKMACODE_DEBUG") == "true"
}

// EnableDebug activa salida adicional por stdout para depuración.
func EnableDebug() {
	debugEnabled = true
}

func Println(v ...interface{}) {
	logger.Println(v...)
	if debugEnabled {
		log.Println(v...)
	}
}

func Printf(format string, v ...interface{}) {
	logger.Printf(format, v...)
	if debugEnabled {
		log.Printf(format, v...)
	}
}
