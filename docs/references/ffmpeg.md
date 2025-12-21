# FFmpeg Reference

FFmpeg is a complete, cross-platform solution to record, convert, stream, and play multimedia content. It provides a vast suite of libraries and programs for handling audio and video.

## Overview

FFmpeg includes:
- **ffmpeg**: Command-line tool for format conversion
- **ffprobe**: Multimedia stream analyzer
- **ffplay**: Simple media player
- **libavcodec**: Encoding/decoding library
- **libavformat**: Muxing/demuxing library
- **libavfilter**: Filtering library
- **libswscale**: Scaling library
- **libswresample**: Audio resampling library

---

## Basic Usage

### General Syntax

```bash
ffmpeg [global_options] {[input_options] -i input_url} ... {[output_options] output_url} ...
```

### Simple Conversion Examples

**Convert audio file to MP2 with specific sample rate:**
```bash
ffmpeg -i /tmp/a.wav -ar 22050 /tmp/a.mp2
```

**Convert video format:**
```bash
ffmpeg -i input.avi output.mp4
```

---

## Stream Selection and Mapping

### Copy Single Stream

Copy a specific stream without re-encoding:

```bash
ffmpeg -i INPUT.mkv -map 0:1 -c copy OUTPUT.mp4
```

### Split Streams to Multiple Outputs

```bash
ffmpeg -i INPUT.mkv -map 0:0 -c copy OUTPUT0.mp4 -map 0:1 -c copy OUTPUT1.mp4
```

### Automatic and Manual Stream Selection

```bash
ffmpeg -i A.avi -i B.mp4 out1.mkv out2.wav -map 1:a -c:a copy out3.mov
```

### Encode Video, Copy Audio

```bash
ffmpeg -i INPUT -map 0 -c:v libx264 -c:a copy OUTPUT
```

### Multiple Codec Specifications

```bash
ffmpeg -i INPUT -map 0 -c copy -c:v:1 libx264 -c:a:137 libvorbis OUTPUT
```

---

## Codec Options

### Stream Copy (No Re-encoding)

Use `-c copy` to copy streams without re-encoding:

```bash
ffmpeg -i input.mkv -c copy output.mp4
```

### Specify Codecs Per Stream

```bash
ffmpeg -i INPUT -c:v libx264 -c:a aac OUTPUT.mp4
```

### Per-Stream Audio Options

```bash
ffmpeg -i multichannel.mxf -map 0:v:0 -map 0:a:0 -map 0:a:0 \
  -c:a:0 ac3 -b:a:0 640k \
  -ac:a:1 2 -c:a:1 aac -b:2 128k \
  out.mp4
```

---

## Two-Pass Encoding

Two-pass encoding provides better bitrate control for target file sizes.

### Pass 1 (Statistics Collection)

**Linux/macOS:**
```bash
ffmpeg -i foo.mov -c:v libxvid -pass 1 -an -f rawvideo -y /dev/null
```

**Windows:**
```bash
ffmpeg -i foo.mov -c:v libxvid -pass 1 -an -f rawvideo -y NUL
```

### Pass 2 (Final Encoding)

```bash
ffmpeg -i foo.mov -c:v libxvid -pass 2 -b:v 2000k output.avi
```

---

## Video Filters

### Scale Filter

**Scale to specific dimensions:**
```bash
ffmpeg -i input.mp4 -vf "scale=1280:720" output.mp4
```

**Scale to double size:**
```bash
ffmpeg -i input.mp4 -vf "scale=2*iw:2*ih" output.mp4
```

**Scale to half size:**
```bash
ffmpeg -i input.mp4 -vf "scale=iw/2:ih/2" output.mp4
```

**Scale width, maintain aspect ratio:**
```bash
ffmpeg -i input.mp4 -vf "scale=640:-1" output.mp4
```

**Max 500 pixels width, preserve aspect ratio:**
```bash
ffmpeg -i input.mp4 -vf "scale=w='min(500,iw*3/2)':h=-1" output.mp4
```

**Force divisible by 2 (for codecs requiring even dimensions):**
```bash
ffmpeg -i input.mp4 -vf "scale=400:300:force_original_aspect_ratio=decrease:force_divisible_by=2" output.mp4
```

### Overlay Filter

Place one video on top of another:

```bash
ffmpeg -i main_video.mp4 -i overlay_video.mp4 \
  -filter_complex "[0:v][1:v]overlay=x=10:y=20" \
  -codec:a copy output.mp4
```

### Tile Filter

Create a grid of video frames:

```bash
ffmpeg -i input.mp4 -vf "tile=3x2:nb_frames=5:padding=7:margin=2" output.png
```

### Stack Videos

Horizontal stack using xstack:

```bash
ffmpeg -i input1.mp4 -i input2.mp4 -filter_complex "xstack=grid=2x1" output.mp4
```

---

## Audio Filters

### Speech Normalization

Normalize audio amplitudes:

```bash
ffmpeg -i input.mp3 -af "speechnorm=e=12.5:r=0.0001:l=1" output.mp3
```

### Rubberband (Time Stretching)

```bash
ffmpeg -i input.mp3 -af "rubberband=transients=crisp:detector=percussive" output.mp3
```

### Equalizer

```bash
ffmpeg -i input.mp3 -af "firequalizer=delay=0.1:fixed=on:zero_phase=on" output.mp3
```

### Audio Tempo Change with Commands

```bash
ffmpeg -i input.mp3 -af "asendcmd=c='4.0 atempo tempo 1.5',atempo" output.mp3
```

---

## Concatenation

### Using concat Filter

Join multiple video files:

```bash
ffmpeg -i opening.mkv -i episode.mkv -i ending.mkv -filter_complex \
  '[0:0] [0:1] [0:2] [1:0] [1:1] [1:2] [2:0] [2:1] [2:2]
   concat=n=3:v=1:a=2 [v] [a1] [a2]' \
  -map '[v]' -map '[a1]' -map '[a2]' output.mkv
```

### Separate Audio/Video Concatenation

```bash
ffmpeg -f lavfi -i "movie=part1.mp4, scale=512:288 [v1]; amovie=part1.mp4 [a1];
movie=part2.mp4, scale=512:288 [v2]; amovie=part2.mp4 [a2];
[v1] [v2] concat [outv]; [a1] [a2] concat=v=0:a=1 [outa]" output.mp4
```

---

## Complex Filtergraphs

### Scale, Encode, and Compare

```bash
ffmpeg -i input.mkv \
  -filter_complex '[0:v]scale=size=hd1080,split=outputs=2[for_enc][orig_scaled]' \
  -c:v libx264 -map '[for_enc]' output.mkv \
  -dec 0:0 \
  -filter_complex '[dec:0][orig_scaled]hstack[stacked]' \
  -map '[stacked]' -c:v ffv1 comparison.mkv
```

### Audio Vector Scope Visualization

```bash
ffplay -f lavfi 'amovie=input.mp3, asplit [a][out1];
  [a] avectorscope=zoom=1.3:rc=2:gc=200:bc=10:rf=1:gf=8:bf=7 [out0]'
```

### Show Spectrum

```bash
ffmpeg -i input.mp3 -filter_complex "showspectrum=s=1280x480:scale=log" output.mp4
```

---

## HLS Streaming

### Create HLS with Multiple Variants

```bash
ffmpeg -re -i in.ts -b:a:0 32k -b:a:1 64k -b:v:0 1000k \
  -map 0:a -map 0:a -map 0:v -f hls \
  -var_stream_map "a:0,agroup:aud_low,default:yes,language:ENG a:1,agroup:aud_low,language:CHN v:0,agroup:aud_low" \
  -master_pl_name master.m3u8 \
  http://example.com/live/out_%v.m3u8
```

---

## Bitstream Filters

### H.264 MP4 to Annex B

Convert H.264 from MP4 format to Annex B:

```bash
ffmpeg -bsf:v h264_mp4toannexb -i h264.mp4 -c:v copy -an out.h264
```

---

## Hardware Acceleration

### Vulkan Filter Example

```bash
ffmpeg -init_hw_device vulkan=vk:0 -filter_hw_device vk \
  -i INPUT -vf "hwupload,nlmeans_vulkan,hwdownload" OUTPUT
```

---

## Timeline Editing

### Enable Filters Based on Time

```bash
ffmpeg -i input.mp4 -vf "smartblur=enable='between(t,10,180)',curves=enable='gte(t,3)':preset=cross_process" output.mp4
```

---

## Quality Measurement

### VMAF Calculation

```bash
ffmpeg -i distorted.mpg -i reference.mkv \
  -lavfi "[0:v]settb=AVTB,setpts=PTS-STARTPTS[main];[1:v]settb=AVTB,setpts=PTS-STARTPTS[ref];[main][ref]libvmaf=log_fmt=json:log_path=output.json" \
  -f null -
```

---

## Metadata and Chapters

### Copy Chapters from Input

```bash
ffmpeg -i input_with_chapters.mkv -map_chapters 0 output.mkv
```

---

## Subtitle Formats

| Format | Muxing | Demuxing | Encoding | Decoding |
|--------|--------|----------|----------|----------|
| SubRip (SRT) | X | X | X | X |
| SSA/ASS | X | X | X | X |
| WebVTT | X | X | X | X |
| DVB | X | X | X | X |
| DVD | X | X | X | X |
| PGS | - | - | - | X |
| TTML | X | - | X | - |
| MicroDVD | X | X | - | X |

### Convert Subtitles

```bash
ffmpeg -i CC.rcwt.bin -c:s copy CC.scc
```

---

## Common Options

### Global Options

| Option | Description |
|--------|-------------|
| `-y` | Overwrite output files without asking |
| `-n` | Do not overwrite output files |
| `-v level` | Set logging verbosity |
| `-hide_banner` | Suppress printing banner |

### Input/Output Options

| Option | Description |
|--------|-------------|
| `-i url` | Input file URL |
| `-f fmt` | Force format |
| `-c codec` | Codec name |
| `-c:v codec` | Video codec |
| `-c:a codec` | Audio codec |
| `-c:s codec` | Subtitle codec |
| `-t duration` | Duration to encode |
| `-ss position` | Start time offset |
| `-to position` | Stop writing at position |

### Video Options

| Option | Description |
|--------|-------------|
| `-r fps` | Frame rate |
| `-s WxH` | Frame size |
| `-vf filter` | Video filter |
| `-b:v bitrate` | Video bitrate |
| `-crf value` | Constant Rate Factor (quality) |
| `-preset name` | Encoding preset |

### Audio Options

| Option | Description |
|--------|-------------|
| `-ar rate` | Audio sample rate |
| `-ac channels` | Number of audio channels |
| `-af filter` | Audio filter |
| `-b:a bitrate` | Audio bitrate |
| `-an` | Disable audio |

### Advanced Options

| Option | Description |
|--------|-------------|
| `-map input:stream` | Stream mapping |
| `-filter_complex graph` | Complex filtergraph |
| `-pass n` | Pass number for two-pass |
| `-max_muxing_queue_size` | Muxing queue buffer size |
| `-reinit_filter` | Reinitialize filters |

---

## Common Audio Codecs

| Codec | Description |
|-------|-------------|
| `aac` | Advanced Audio Coding (native) |
| `libmp3lame` | MP3 encoder |
| `libopus` | Opus encoder |
| `libvorbis` | Vorbis encoder |
| `ac3` | Dolby Digital |
| `flac` | Free Lossless Audio Codec |
| `pcm_s16le` | PCM signed 16-bit little-endian |

---

## Common Video Codecs

| Codec | Description |
|-------|-------------|
| `libx264` | H.264/AVC encoder |
| `libx265` | H.265/HEVC encoder |
| `libvpx` | VP8 encoder |
| `libvpx-vp9` | VP9 encoder |
| `libaom-av1` | AV1 encoder |
| `libxvid` | MPEG-4 Part 2 |
| `ffv1` | FFmpeg Video Codec 1 (lossless) |

---

## Useful Command Patterns

### Extract Audio from Video

```bash
ffmpeg -i video.mp4 -vn -c:a copy audio.m4a
```

### Extract Video without Audio

```bash
ffmpeg -i video.mp4 -an -c:v copy video_only.mp4
```

### Convert to MP3

```bash
ffmpeg -i input.wav -c:a libmp3lame -b:a 320k output.mp3
```

### Create Thumbnail

```bash
ffmpeg -i video.mp4 -ss 00:00:05 -vframes 1 thumbnail.jpg
```

### Get Media Information

```bash
ffprobe -v quiet -print_format json -show_format -show_streams input.mp4
```

---

## Resources

- [FFmpeg Official Documentation](https://ffmpeg.org/documentation.html)
- [FFmpeg Wiki](https://trac.ffmpeg.org/wiki)
- [FFmpeg Filters Documentation](https://ffmpeg.org/ffmpeg-filters.html)
- [FFmpeg Codecs Documentation](https://ffmpeg.org/ffmpeg-codecs.html)
