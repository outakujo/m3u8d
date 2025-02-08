### m3u8下载工具

#### Install

```
go install github.com/outakujo/m3u8d@latest
```

#### Use

* 下载ts文件

m3u8链接中包含&符号，则需要加上双引号，没有则可以不加

```
m3u8d -i "m3u8链接"
```

网络原因可能会存在部分文件下载失败，可以多次执行，直到errNum为0，即其全部下载成功

```
download succNum: 256 errNum: 0 cost: 1m32.494006s
```

* 合并ts到mp4，需要依赖ffmpeg

```
m3u8d -i "m3u8链接" -mp4
```

#### Param:

-gen generate new index.m3u8

-i string m3u8 url or file

-mfn string mp4 file name (default "out")

-mp int max parallel (default 5)

-mp4 ffmpeg out mp4

-o only download m3u8 file

-st int single request timeout(seconds) (default 5)

-up string m3u8 url prefix

-v verbose

-wk string work dir (default "m3u8cache")
