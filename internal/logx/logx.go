package logx

import "log"

func Info(tag, format string, args ...any) {
	log.Printf("["+tag+"] "+format, args...)
}

func Error(tag, format string, args ...any) {
	log.Printf("["+tag+"] ERROR "+format, args...)
}

func Warn(tag, format string, args ...any) {
	log.Printf("["+tag+"] WARN "+format, args...)
}
