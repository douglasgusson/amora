package cli

import "fmt"

// ANSI color codes for terminal output.
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Gray    = "\033[90m"
)

// LogInfo prints an informational message with a blue arrow prefix.
func LogInfo(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s%s-----> %s%s\n", Blue, Bold, msg, Reset)
}

// LogSuccess prints a success message with a green checkmark prefix.
func LogSuccess(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s%s       ✓ %s%s\n", Green, Bold, msg, Reset)
}

// LogError prints an error message with a red exclamation prefix.
func LogError(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s%s !     %s%s\n", Red, Bold, msg, Reset)
}

// LogStream prints a line of command output with a gray prefix,
// used for streaming subprocess output in real-time.
func LogStream(line string) {
	fmt.Printf("%s       %s%s\n", Gray, line, Reset)
}

// Banner prints the Amora logo/title.
func Banner() {
	fmt.Printf("\n%s%s🍇 Amora%s\n\n", Magenta, Bold, Reset)
}
