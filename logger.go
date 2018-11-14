package roshi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)


const (
	L_NOTICE_ACCESS = iota
	L_NOTICE_ERROR
)

const (
	L_FORMAT_ACCESS = "[%s] [%s] \"%s %s %s %s\" %d %d"
	L_FORMAT_ERROR  = "[%s] [%s] %s: %s"
	L_FORMAT_NOTICE = "[%s] NOTICE: %s\n"
)


type Logger interface {
    Access(r *http.Request, status, length int64)
	Error(r *http.Request, err error)
	Notice(message string, log ...int)
    Close()
}


type RoshiLogger struct {
	errorPath  string
	accessPath string

	errorFh  *os.File
	accessFh *os.File

	errorQ  chan string
	accessQ chan string

	wg         sync.WaitGroup
	inShutdown bool

	BlockingWrite bool
}


// create a new logger instance
func NewLogger(errorPath, accessPath string, qSize int) (logger *RoshiLogger, err error) {
	logger = &RoshiLogger{
		errorPath:  errorPath,
		accessPath: accessPath,
		errorQ:     make(chan string, qSize),
		accessQ:    make(chan string, qSize),
	}

	if err = logger.openError(); nil != err {
		return
	}

	if err = logger.openAccess(); nil != err {
		return
	}

	go logger.accessConsumer()
	go logger.errorConsumer()

	return
}




func (l *RoshiLogger) Access(r *http.Request, status, length int64) {
	log :=  fmt.Sprintf(
		L_FORMAT_ACCESS,
		formattedDate(),
		r.RemoteAddr,
		r.Method,
		r.RequestURI,
		r.Proto,
		r.Header.Get("User-Agent"),
		status,
		length,
	)

	if l.BlockingWrite {
		l.accessQ <- log
	} else {
		select {
		case l.accessQ <- log:
		default:
		}
	}

}


func (l *RoshiLogger) Error(r *http.Request, err error) {
	log :=  fmt.Sprintf(
		L_FORMAT_ERROR,
		formattedDate(),
		r.RemoteAddr,
		r.RequestURI,
		err.Error(),
	)

	if l.BlockingWrite {
		l.errorQ <- log
	} else {
		select {
		case l.errorQ <- log:
		default:
		}
	}

}


func (l *RoshiLogger) Notice(message string, log ...int) {
	ltype := L_NOTICE_ACCESS
	if 0 < len(log) {
		ltype = log[0]
	}

	var q chan string
	if L_NOTICE_ACCESS == ltype {
		q = l.accessQ
	} else {
		q = l.errorQ
	}

	if l.BlockingWrite {
		q <- fmt.Sprintf(L_FORMAT_NOTICE, formattedDate(), message)
	} else {
		select {
		case l.errorQ <- fmt.Sprintf(L_FORMAT_NOTICE, formattedDate(), message):
		default:
		}
	}
}


func (l *RoshiLogger) Close() {
	l.Notice("Server Shutting Down")
	l.inShutdown = true

	close(l.errorQ)
	close(l.accessQ)

	l.wg.Wait()
	l.accessFh.Close()
	l.errorFh.Close()
}


func (l *RoshiLogger) openError() (err error) {
	l.errorFh, err = openFile(l.errorPath)
	return
}


func (l *RoshiLogger) openAccess() (err error) {
	l.accessFh, err = openFile(l.accessPath)
	return
}


func (l *RoshiLogger) errorConsumer() {
	for log := range l.errorQ {
		if nil == l.errorFh {
			l.openError()
		}

		if _, err := l.errorFh.WriteString(log + "\n"); nil != err {
			l.openError()
			l.errorFh.WriteString(log + "\n")
		}
	}
}


func (l *RoshiLogger) accessConsumer() {
	for log := range l.accessQ {
		if nil == l.accessFh {
			l.openAccess()
		}

		if _, err := l.accessFh.WriteString(log + "\n"); nil != err {
			l.openAccess()
			l.accessFh.WriteString(log + "\n")
		}
	}
}


// check path and open a file
func openFile(path string) (handle *os.File, err error) {
	if path, err = filepath.Abs(path); nil != err {
		return
	}

	handle, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	return
}


func formattedDate() string {
	return time.Now().Format(time.RFC1123Z)
}