package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	var ur string
	var wk string
	var maxParallel int
	var verbose bool
	var only bool
	var mp4 bool
	flag.StringVar(&ur, "ur", "", "m3u8 url")
	flag.StringVar(&wk, "wk", "m3u8cache", "work dir")
	flag.IntVar(&maxParallel, "mp", 5, "max parallel")
	flag.BoolVar(&verbose, "v", false, "verbose")
	flag.BoolVar(&only, "o", false, "only download m3u8 file")
	flag.BoolVar(&mp4, "mp4", false, "ffmpeg out mp4")
	flag.Parse()
	if ur == "" {
		log.Printf("m3u8 url not be empty\n")
		return
	}
	_ = os.MkdirAll(wk, os.ModePerm)
	urPrefix, err := UriPrefix(ur)
	if err != nil {
		log.Printf("UriPrefix %v\n", err)
		return
	}
	if only {
		_, fn, _ := strings.Cut(ur, urPrefix+"/")
		if fn != "" {
			err := DownloadFile(ur, wk+"/"+fn, 0)
			if err != nil {
				log.Printf("DownloadFile %v %v\n", ur, err)
			}
		}
		return
	}
	bs, err := DownloadFileBytes(ur, 0)
	if err != nil {
		log.Printf("DownloadFileBytes %v\n", err)
		return
	}
	m3u8, err := ParseM3u8(bs, urPrefix)
	if err != nil {
		log.Printf("ParseM3u8 %v\n", err)
		return
	}
	if len(m3u8.Key) != 0 {
		log.Printf("key: %v iv: 0x%v tsSum: %v\n",
			hex.EncodeToString(m3u8.Key), hex.EncodeToString(m3u8.Iv),
			len(m3u8.Tss))
	} else {
		log.Printf("tsSum: %v\n", len(m3u8.Tss))
	}
	loader := NewLoader(maxParallel, verbose)
	fisn := wk + "/files.txt"
	files, err := os.OpenFile(fisn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		log.Printf("create files %v\n", err)
		return
	}
	defer files.Close()
	fsm := make(map[int]string)
	for i, t := range m3u8.Tss {
		go func(ind int, ts Ts) {
			var afc = func(w Work, data []byte) (err error) {
				dst := data
				if len(m3u8.Key) != 0 {
					dst, err = AesDecryptByCBC(data, m3u8.Key, m3u8.Iv)
					if err != nil {
						err = fmt.Errorf("AesDecryptByCBC %v", err)
						return
					}
				}
				sfn := wk + "/" + ts.Name + ".ts"
				err = os.WriteFile(sfn, dst, os.ModePerm)
				if err != nil {
					err = fmt.Errorf("WriteFile %v", err)
				} else {
					var abs string
					abs, err = filepath.Abs(sfn)
					if err != nil {
						err = fmt.Errorf("file abs %v", err)
						return
					}
					fsm[ind] = fmt.Sprintf("file '%v'\n", abs)
				}
				return
			}
			loader.Do(Work{Ur: m3u8.UrPrefix + "/" + ts.Value, AfterFun: afc})
		}(i, t)
	}
	loader.Wait()
	succ, errn, cost := loader.Stat()
	log.Printf("download succNum: %v errNum: %v cost: %s\n", succ, errn, cost)
	ln := len(fsm)
	if ln == 0 {
		return
	}
	for i := 0; i < ln; i++ {
		_, _ = files.WriteString(fsm[i])
	}
	abs, err := filepath.Abs(fisn)
	if err != nil {
		log.Printf("file abs %v %v\n", fisn, abs)
		return
	}
	fcp := []string{"-f", "concat", "-safe", "0",
		"-i", abs, "-c", "copy", "out.mp4"}
	fcpj := strings.Join(fcp, " ")
	if mp4 {
		log.Printf("ffmpeg %v\n", fcpj)
		command := exec.Command("ffmpeg", fcp...)
		command.Stdout = os.Stdout
		err = command.Run()
		if err != nil {
			log.Printf("exec ffmpeg %v\n", err)
		} else {
			time.Sleep(time.Second)
			_ = os.RemoveAll(wk)
		}
	} else {
		log.Printf("please run\nffmpeg %v\n", fcpj)
	}
}
