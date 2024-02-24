package main

import "fmt"

func wrapWithRed(message string) string {
	return fmt.Sprintf("\033[31m%s\033[0m", message)
}

func wrapWithGreen(message string) string {
	return fmt.Sprintf("\033[32m%s\033[0m", message)
}

func wrapWithColor(message, color string) string {
	return fmt.Sprintf("\033[%sm%s\033[0m", color, message)
}

func wrapWithBold(message string) string {
	return fmt.Sprintf("\033[1m%s\033[0m", message)
}
