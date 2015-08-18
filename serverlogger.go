package reverseproxy
import (
	"io"
	"log"
)

const (
	LevelDebug = 1 << iota
	LevelInfo
	LevelWarning
	LevelError
)

var (
	l *log.Logger
	logFlag uint8
)

func InitLog(flag uint8, writer io.Writer) {
	logFlag = flag
	l = log.New(writer, "", log.Ldate|log.Ltime)
}

func Debug(v ...interface{}) {
	if l != nil && logFlag & LevelDebug > 0 {
		l.Println(append([]interface{}{"DEBUG"}, v...))
	}
}

func Info(v ...interface{}) {
	if l != nil && logFlag & LevelInfo > 0 {
		l.Println("INFO:", v)
	}
}

func Warning(v ...interface{}) {
	if l != nil && logFlag & LevelWarning > 0 {
		l.Println("WARNING:", v)
	}
}

func Error(v ...interface{}) {
	if l != nil && logFlag & LevelError > 0 {
		l.Println("ERRORexp:", v)
	}
}
