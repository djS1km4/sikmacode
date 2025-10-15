package log

import (
	"log"
	"os"
)

var logger *log.Logger

func init() {
	file, err := os.OpenFile("sikmacode_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	logger = log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func Println(v ...interface{}) {
	logger.Println(v...)
}

func Printf(format string, v ...interface{}) {
	logger.Printf(format, v...)
}
