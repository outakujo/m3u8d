### m3u8下载工具

#### Install

```
go install github.com/outakujo/m3u8d@latest
```

#### Use

```
m3u8d -mp4 -ur m3u8链接
```

**m3u8链接中包含&符号，则需要加上双引号**

```
m3u8d -mp4 -ur "m3u8链接"
```

#### Param:

-mp int max parallel (default 5)

-mp4 ffmpeg out mp4

-o  only download m3u8 file

-ur string m3u8 url

-v  verbose

-wk string work dir (default "m3u8cache")
