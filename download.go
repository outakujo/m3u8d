package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"
)

func DownloadFile(ur, fn string, timeout time.Duration) error {
	cli := &http.Client{}
	cli.Timeout = timeout
	get, err := cli.Get(ur)
	if err != nil {
		return err
	}
	defer get.Body.Close()
	file, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(get.Body)
	_, err = reader.WriteTo(file)
	return err
}

func DownloadFileBytes(ur string, timeout time.Duration) ([]byte, error) {
	cli := &http.Client{}
	cli.Timeout = timeout
	get, err := cli.Get(ur)
	if err != nil {
		return nil, err
	}
	defer get.Body.Close()
	return io.ReadAll(get.Body)
}

func ParseM3u8(m3u8Bytes []byte, urPrefix string) (key, iv []byte, tss []string, err error) {
	reg := `#EXT-X-KEY:METHOD=AES-128,URI="(.*)",IV=0x(.*)`
	regTs := `([0-9]*)[.]ts`
	regInf := `#EXTINF:(.*)`
	keyCompile, err := regexp.Compile(reg)
	if err != nil {
		return
	}
	tsCompile, err := regexp.Compile(regTs)
	if err != nil {
		return
	}
	infCompile, err := regexp.Compile(regInf)
	if err != nil {
		return
	}
	reader := bytes.NewReader(m3u8Bytes)
	scanner := bufio.NewScanner(reader)
	tss = make([]string, 0)
	key = make([]byte, 0)
	iv = make([]byte, 0)
	for scanner.Scan() {
		text := scanner.Text()
		submatch := keyCompile.FindStringSubmatch(text)
		if len(submatch) == 3 {
			keys := submatch[1]
			if keys != "" {
				urKey := keys
				match, _ := regexp.MatchString("http(s?)://(.*)", keys)
				if !match {
					urKey = urPrefix + "/" + keys
				}
				key, err = DownloadFileBytes(urKey, 0)
				if err != nil {
					return
				}
			}
			ivs := submatch[2]
			if ivs != "" {
				iv, err = hex.DecodeString(ivs)
				if err != nil {
					return
				}
			}
		}
		if infCompile.MatchString(text) {
			break
		}
	}
	for scanner.Scan() {
		text := scanner.Text()
		ts := tsCompile.FindString(text)
		if ts != "" {
			tss = append(tss, ts)
		}
	}
	return
}

type Work struct {
	Ur       string
	Timeout  time.Duration
	AfterFun func(w Work, data []byte)
}

type Loader struct {
	maxParallel int
	workc       chan struct{}
	verbose     bool
	succNum     int
	errNum      int
	mut         sync.Mutex
	wait        sync.WaitGroup
	startTime   time.Time
	endTime     time.Time
}

func NewLoader(maxParallel int, verbose bool) *Loader {
	if maxParallel == 0 {
		maxParallel = 5
	}
	l := &Loader{maxParallel: maxParallel, verbose: verbose}
	l.workc = make(chan struct{}, l.maxParallel)
	return l
}

func (l *Loader) Do(w Work) {
	l.wait.Add(1)
	l.workc <- struct{}{}
	go func() {
		fileBytes, err := DownloadFileBytes(w.Ur, w.Timeout)
		go func() {
			l.mut.Lock()
			defer l.mut.Unlock()
			if err != nil {
				l.errNum++
				if l.verbose {
					log.Printf("DownloadFile %v %v\n", w.Ur, err)
				}
			} else {
				l.succNum++
				if w.AfterFun != nil {
					w.AfterFun(w, fileBytes)
				}
			}
			l.wait.Done()
		}()
		<-l.workc
	}()
}

func (l *Loader) Stat() (succ, err int, dur time.Duration) {
	l.endTime = time.Now()
	return l.succNum, l.errNum, l.endTime.Sub(l.startTime)
}

func (l *Loader) ResetStat() {
	l.mut.Lock()
	defer l.mut.Unlock()
	l.succNum = 0
	l.errNum = 0
	l.startTime = time.Time{}
	l.endTime = time.Time{}
}

func (l *Loader) Wait() {
	l.startTime = time.Now()
	l.wait.Wait()
}
