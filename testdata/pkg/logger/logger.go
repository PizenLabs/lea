package logger

import "fmt"

type Logger struct{}

func New() *Logger { return &Logger{} }

func (l *Logger) Info(msg string) {
	fmt.Printf("[INFO] %s\n", msg)
}
