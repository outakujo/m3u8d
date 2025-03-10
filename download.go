package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

func DownloadFile(ur, fn, jsonHeader string, timeout time.Duration) error {
	cli := &http.Client{}
	cli.Timeout = timeout
	req, err := http.NewRequest(http.MethodGet, ur, nil)
	if err != nil {
		return err
	}
	if jsonHeader != "" {
		req.Header = make(http.Header)
		heda := make(map[string]string)
		err = json.Unmarshal([]byte(jsonHeader), &heda)
		if err != nil {
			return err
		}
		for k, v := range heda {
			req.Header.Set(k, v)
		}
	}
	get, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = get.Body.Close()
	}()
	if get.StatusCode != http.StatusOK {
		return fmt.Errorf("httpcode be %v", get.StatusCode)
	}
	file, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(get.Body)
	_, err = reader.WriteTo(file)
	return err
}

func DownloadFileBytes(ur, jsonHeader string, timeout time.Duration) ([]byte, error) {
	cli := &http.Client{}
	cli.Timeout = timeout
	req, err := http.NewRequest(http.MethodGet, ur, nil)
	if err != nil {
		return nil, err
	}
	if jsonHeader != "" {
		req.Header = make(http.Header)
		heda := make(map[string]string)
		err = json.Unmarshal([]byte(jsonHeader), &heda)
		if err != nil {
			return nil, err
		}
		for k, v := range heda {
			req.Header.Set(k, v)
		}
	}
	get, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = get.Body.Close()
	}()
	if get.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("httpcode be %v", get.StatusCode)
	}
	return io.ReadAll(get.Body)
}

type M3u8 struct {
	Key      []byte
	Iv       []byte
	Tss      []Ts
	UrPrefix string
}

type Ts struct {
	Name      string
	Value     string
	EXTINF    string
	IsDecrypt bool
}

func ParseM3u8(m3u8Bytes []byte, urPrefix, header string) (m3u8 M3u8, err error) {
	reg := `#EXT-X-KEY:METHOD=AES-128,URI="(.*)"(,IV=0x(.*))?`
	regTs := `(.*)[.](ts|jpg|jpeg|jfif|pjpeg|pjp|png|webp|gif)`
	regInf := `#EXTINF:(.*)`
	streamInf := `#EXT-X-STREAM-INF:(.*)`
	notMethod := `#EXT-X-KEY:METHOD=NONE`
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
	streamInfCompile, err := regexp.Compile(streamInf)
	if err != nil {
		return
	}
	notMethodCompile, err := regexp.Compile(notMethod)
	if err != nil {
		return
	}
	reader := bytes.NewReader(m3u8Bytes)
	scanner := bufio.NewScanner(reader)
	m3u8.Tss = make([]Ts, 0)
	m3u8.Key = make([]byte, 0)
	m3u8.Iv = make([]byte, 0)
	m3u8.UrPrefix = urPrefix
	var getkey = func(text string) (find bool) {
		text = strings.TrimSpace(text)
		submatch := keyCompile.FindStringSubmatch(text)
		ln := len(submatch)
		if ln < 2 {
			return
		}
		keys := strings.TrimSpace(submatch[1])
		if keys != "" {
			m3u8.Key, err = DownloadFileBytes(UriAbs(keys, urPrefix), header, 0)
			if err != nil {
				return
			} else {
				find = true
			}
		}
		if ln < 4 {
			return
		}
		ivs := strings.TrimSpace(submatch[3])
		if ivs != "" {
			m3u8.Iv, err = hex.DecodeString(ivs)
			if err != nil {
				return
			}
		}
		return
	}
	isDecrypt := false
	for scanner.Scan() {
		text := scanner.Text()
		if streamInfCompile.MatchString(text) {
			if scanner.Scan() {
				text = scanner.Text()
				if text == "" {
					err = errors.New("parse STREAM-INF failed")
					return
				}
				var nm3u8bs []byte
				nm3u8bs, err = DownloadFileBytes(UriAbs(text, urPrefix), header, 0)
				if err != nil {
					return
				}
				var ufa string
				ufa, err = UriPrefix(text)
				if err != nil {
					return
				}
				if ufa != "" {
					urPrefix = urPrefix + "/" + ufa
					m3u8.UrPrefix = urPrefix
				}
				return ParseM3u8(nm3u8bs, urPrefix, header)
			}
			return
		}
		if notMethodCompile.MatchString(text) {
			isDecrypt = false
		}
		if getkey(text) {
			isDecrypt = true
		}
		if infCompile.MatchString(text) {
			exinf := text
			if !scanner.Scan() {
				continue
			}
			tst := strings.TrimSpace(scanner.Text())
			ts := tsCompile.FindString(tst)
			if ts == "" {
				continue
			}
			index := strings.LastIndex(ts, "/")
			if index != -1 {
				ts = ts[index+1:]
			}
			m3u8.Tss = append(m3u8.Tss, Ts{
				Name:      strings.Split(ts, ".")[0],
				Value:     UriAbs(tst, urPrefix),
				EXTINF:    exinf,
				IsDecrypt: isDecrypt,
			})
		}
	}
	return
}

func UriAbs(uri, urPrefix string) string {
	if IsHttp(uri) {
		return uri
	}
	if string(uri[0]) == "/" {
		parse, err := url.Parse(urPrefix)
		if err != nil {
			return ""
		}
		return fmt.Sprintf("%v://%v%v", parse.Scheme, parse.Host, uri)
	}
	return urPrefix + "/" + uri
}

func IsHttp(uri string) bool {
	match, _ := regexp.MatchString("http(s?)://(.*)", uri)
	return match
}

func UriPrefix(uri string) (pf string, err error) {
	if !strings.Contains(uri, "/") {
		return
	}
	reg := `(.*)/.*[.]m3u8`
	m3u8Compile, err := regexp.Compile(reg)
	if err != nil {
		return
	}
	submatch := m3u8Compile.FindStringSubmatch(uri)
	if len(submatch) != 2 {
		err = errors.New("not be m3u8 url")
		return
	}
	pf = submatch[1]
	return
}

type Work struct {
	Ur       string
	Header   string
	Timeout  time.Duration
	AfterFun func(w Work, data []byte) error
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
	if l.startTime.IsZero() {
		l.startTime = time.Now()
	}
	go func() {
		fileBytes, err := DownloadFileBytes(w.Ur, w.Header, w.Timeout)
		go func() {
			l.mut.Lock()
			defer l.mut.Unlock()
			if err != nil {
				l.errNum++
				if l.verbose {
					log.Printf("DownloadFile %v %v\n", w.Ur, err)
				}
			} else {
				if w.AfterFun != nil {
					err = w.AfterFun(w, fileBytes)
					if err != nil {
						l.errNum++
						if l.verbose {
							log.Printf("AfterFun %v %v\n", w.Ur, err)
						}
					} else {
						l.succNum++
					}
				}
			}
			l.endTime = time.Now()
			l.wait.Done()
		}()
		<-l.workc
	}()
}

func (l *Loader) Stat() (succ, err int, dur time.Duration) {
	var cost time.Duration
	if !l.endTime.IsZero() {
		cost = l.endTime.Sub(l.startTime)
	}
	return l.succNum, l.errNum, cost
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
	l.wait.Wait()
}
