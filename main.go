package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	wk            string
	maxParallel   int
	verbose       bool
	singleTimeout int
)

func main() {
	var ir string
	var only bool
	var mp4 bool
	var gen bool
	var mp4fn string
	var urPrefix string
	var jsonHeader string
	flag.StringVar(&ir, "i", "", "m3u8 url or file")
	flag.StringVar(&wk, "wk", "m3u8cache", "work dir")
	flag.StringVar(&urPrefix, "up", "", "m3u8 url prefix")
	flag.StringVar(&jsonHeader, "jh", "", "json request header")
	flag.StringVar(&mp4fn, "mfn", "out", "mp4 file name")
	flag.IntVar(&maxParallel, "mp", 5, "max parallel")
	flag.IntVar(&singleTimeout, "st", 5, "single request timeout(seconds)")
	flag.BoolVar(&verbose, "v", false, "verbose")
	flag.BoolVar(&only, "o", false, "only download m3u8 file")
	flag.BoolVar(&mp4, "mp4", false, "ffmpeg out mp4")
	flag.BoolVar(&gen, "gen", false, "generate new index.m3u8")
	flag.Parse()
	if ir == "" {
		log.Printf("m3u8 url or file not be empty\n")
		os.Exit(1)
	}
	if jsonHeader != "" {
		heda := make(map[string]string)
		err := json.Unmarshal([]byte(jsonHeader), &heda)
		if err != nil {
			log.Printf("request header not be json %v\n", err)
			os.Exit(2)
		}
	}
	_ = os.MkdirAll(wk, os.ModePerm)
	if only {
		err := onlyDown(ir, jsonHeader)
		if err != nil {
			log.Printf("only download %v\n", err)
			os.Exit(3)
		}
		os.Exit(0)
	}
	files, err := ParseDown(ir, urPrefix, jsonHeader, gen)
	if err != nil {
		log.Printf("parse down %v\n", err)
		os.Exit(4)
	}
	if gen {
		os.Exit(0)
	}
	err = MergeMp4(files, mp4fn, mp4)
	if err != nil {
		log.Printf("merge mp4 %v\n", err)
		os.Exit(5)
	}
}

func onlyDown(ir, header string) error {
	prefix, err := UriPrefix(ir)
	if err != nil {
		return fmt.Errorf("UriPrefix %v", err)
	}
	_, fn, _ := strings.Cut(ir, prefix+"/")
	if fn != "" {
		err = DownloadFile(ir, header, wk+"/"+fn, time.Duration(singleTimeout)*time.Second)
		if err != nil {
			return fmt.Errorf("DownloadFile %v %v", ir, err)
		}
	}
	return nil
}

func getInputBytes(ir, urPrefix, header string) (m3u8bs []byte, prefix string, err error) {
	var m3u8f string
	if !IsHttp(ir) {
		if !FileIsExist(ir) {
			err = fmt.Errorf("m3u8 file not exist")
			return
		}
		m3u8f = ir
		prefix = urPrefix
		if prefix == "" {
			err = fmt.Errorf("m3u8 url prefix not be empty")
			return
		}
	} else {
		prefix, err = UriPrefix(ir)
		if err != nil {
			err = fmt.Errorf("UriPrefix %v", err)
			return
		}
	}
	if m3u8f == "" {
		m3u8bs, err = DownloadFileBytes(ir, header, time.Duration(singleTimeout)*time.Second)
	} else {
		m3u8bs, err = os.ReadFile(m3u8f)
	}
	return
}

func MergeMp4(files, out string, flag bool) error {
	fcp := []string{"-f", "concat", "-safe", "0",
		"-i", files, "-c", "copy", out + ".mp4"}
	fcpj := strings.Join(fcp, " ")
	if flag {
		log.Printf("ffmpeg %v\n", fcpj)
		command := exec.Command("ffmpeg", fcp...)
		command.Stdout = os.Stdout
		err := command.Run()
		if err != nil {
			return err
		} else {
			time.Sleep(time.Second)
			_ = os.RemoveAll(wk)
		}
	} else {
		log.Printf("please run\nffmpeg %v\n", fcpj)
	}
	return nil
}

func ParseDown(ir, urPrefix, header string, genIndex bool) (files string, err error) {
	m3u8bs, prefix, err := getInputBytes(ir, urPrefix, header)
	if err != nil {
		return
	}
	m3u8, err := ParseM3u8(m3u8bs, prefix, header)
	if err != nil {
		return
	}
	if genIndex {
		err = genM3u8Index(m3u8.Tss)
		return
	}
	if len(m3u8.Key) != 0 {
		as := fmt.Sprintf("key: %v", hex.EncodeToString(m3u8.Key))
		if len(m3u8.Iv) != 0 {
			as = fmt.Sprintf("%v iv: 0x%v", as, hex.EncodeToString(m3u8.Iv))
		}
		log.Printf("%v tsSum: %v\n", as, len(m3u8.Tss))
	} else {
		log.Printf("tsSum: %v\n", len(m3u8.Tss))
	}
	loader := NewLoader(maxParallel, verbose)
	fisn := wk + "/files.txt"
	var filestxt *os.File
	filestxt, err = os.OpenFile(fisn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return
	}
	defer func() {
		_ = filestxt.Close()
	}()
	fsm := make(map[int]string)
	for i, t := range m3u8.Tss {
		fn := wk + "/" + t.Name + ".ts"
		if FileIsExist(fn) {
			err = saveToMap(i, fn, fsm)
			if err != nil && verbose {
				log.Printf("saveToMap %v\n", err)
			}
			continue
		}
		func(ind int, ts Ts) {
			var afc = func(w Work, data []byte) (err error) {
				dst := data
				if ts.IsDecrypt && len(m3u8.Key) != 0 {
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
					err = saveToMap(ind, sfn, fsm)
				}
				return
			}
			loader.Do(Work{Ur: ts.Value, Header: header, AfterFun: afc,
				Timeout: time.Duration(singleTimeout) * time.Second})
		}(i, t)
	}
	loader.Wait()
	succ, errn, cost := loader.Stat()
	log.Printf("download succNum: %v errNum: %v cost: %s\n", succ, errn, cost)
	ln := len(fsm)
	if ln == 0 {
		err = errors.New("ts file list be empty")
		return
	}
	for i := 0; i < ln; i++ {
		_, _ = filestxt.WriteString(fsm[i])
	}
	files, err = filepath.Abs(fisn)
	if err != nil {
		err = fmt.Errorf("file abs %v %v", fisn, err)
	}
	return
}

func genM3u8Index(tss []Ts) error {
	prefix := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:4
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD`
	file, err := os.OpenFile(wk+"/index.m3u8", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	_, _ = file.WriteString(prefix + "\n")
	for _, t := range tss {
		_, _ = file.WriteString(t.EXTINF + "\n")
		_, _ = file.WriteString(t.Name + ".ts\n")
	}
	_, _ = file.WriteString("#EXT-X-ENDLIST\n")
	return nil
}

func FileIsExist(fn string) bool {
	_, err := os.Stat(fn)
	return !os.IsNotExist(err)
}

func saveToMap(ind int, fn string, m map[int]string) error {
	abs, err := filepath.Abs(fn)
	if err != nil {
		return fmt.Errorf("file abs %v", err)
	}
	m[ind] = fmt.Sprintf("file '%v'\n", abs)
	return nil
}
