package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	var ur string
	var wk string
	var maxParallel int
	var verbose bool
	var only bool
	flag.StringVar(&ur, "ur", "", "m3u8 url")
	flag.StringVar(&wk, "wk", "m3u8cache", "work dir")
	flag.IntVar(&maxParallel, "mp", 5, "max parallel")
	flag.BoolVar(&verbose, "v", false, "verbose")
	flag.BoolVar(&only, "o", false, "only download m3u8 file")
	flag.Parse()
	if ur == "" {
		log.Printf("m3u8 url not be empty\n")
		return
	}
	_ = os.MkdirAll(wk, os.ModePerm)
	reg := `(.*)/.*[.]m3u8`
	m3u8Compile, err := regexp.Compile(reg)
	if err != nil {
		log.Printf("Compile reg: %v %v\n", reg, err)
		return
	}
	submatch := m3u8Compile.FindStringSubmatch(ur)
	if len(submatch) != 2 {
		log.Printf("not be m3u8 url\n")
		return
	}
	urPrefix := submatch[1]
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
	key, iv, tss, err := ParseM3u8(bs, urPrefix)
	if err != nil {
		log.Printf("ParseM3u8 %v\n", err)
		return
	}
	if len(key) != 0 {
		log.Printf("key: %v iv: 0x%v tsSum: %v\n",
			hex.EncodeToString(key), hex.EncodeToString(iv), len(tss))
	} else {
		log.Printf("tsSum: %v\n", len(tss))
	}
	loader := NewLoader(maxParallel, verbose)
	fisn := wk + "/files.txt"
	files, err := os.OpenFile(fisn, os.O_CREATE, os.ModePerm)
	if err != nil {
		log.Printf("create files %v\n", err)
		return
	}
	defer files.Close()
	fsm := make(map[int]string)
	for i, s := range tss {
		go func(ind int, ts string) {
			loader.Do(Work{Ur: urPrefix + "/" + ts, AfterFun: func(w Work, data []byte) {
				dst := data
				var err error
				if len(key) != 0 {
					dst, err = AesDecryptByCBC(data, key, iv)
					if err != nil && verbose {
						log.Printf("AesDecryptByCBC %v %v\n", w.Ur, err)
						return
					}
				}
				sfn := wk + "/" + ts
				err = os.WriteFile(sfn, dst, os.ModePerm)
				if err != nil && verbose {
					log.Printf("WriteFile %v %v\n", w.Ur, err)
				} else {
					abs, err := filepath.Abs(sfn)
					if err != nil {
						log.Printf("file abs %v %v\n", sfn, abs)
						return
					}
					fsm[ind] = fmt.Sprintf("file '%v'\n", abs)
				}
			}})
		}(i, s)
	}
	loader.Wait()
	succ, errn, cost := loader.Stat()
	log.Printf("download succNum: %v errNum: %v cost: %s\n", succ, errn, cost)
	ln := len(fsm)
	for i := 0; i < ln; i++ {
		_, _ = files.WriteString(fsm[i])
	}
	abs, err := filepath.Abs(fisn)
	if err != nil {
		log.Printf("file abs %v %v\n", fisn, abs)
		return
	}
	log.Printf("please run\nffmpeg -f concat -safe 0 -i %v -c copy out.mp4", abs)
}
