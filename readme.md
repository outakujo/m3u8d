### m3u8下载工具

#### Install

```
go install github.com/outakujo/m3u8d@latest
```

#### Use

```
m3u8d -mp4 -i m3u8链接
```

**m3u8链接中包含&符号，则需要加上双引号**

```
m3u8d -mp4 -i "m3u8链接"
```

#### Param:

-i string m3u8 url or file

-mfn string mp4 file name (default "out")

-mp int max parallel (default 5)

-mp4 ffmpeg out mp4

-o only download m3u8 file

-st int single request timeout(seconds) (default 5)

-up string m3u8 url prefix

-v verbose

-wk string work dir (default "m3u8cache")
