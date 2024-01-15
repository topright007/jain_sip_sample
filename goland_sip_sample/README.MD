```bash
#generate audio
ffplay -nodisp -autoexit -f lavfi -i  '
sine=frequency=440[a];
sine=frequency=587.33[b];
sine=frequency=659.25[c];
[a][b][c]amix=inputs=3,atrim=end=2
'
  
ffmpeg -filter_complex  '
sine=frequency=440[a];
sine=frequency=587.33[b];
sine=frequency=659.25[c];
[a][b][c]amix=inputs=3,atrim=end=2
' -c:a libopus -page_duration 20000 -vn testsrc.ogg
#generate video
 ffmpeg -f lavfi -i testsrc=duration=60:size=1280x720:rate=30 -c:v libvpx testsrc.ivf
```