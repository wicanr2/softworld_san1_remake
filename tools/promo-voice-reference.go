//go:build ignore

// Export the original 361/499/499 spoken tuple for trailer audio verification.
package main

import (
	"encoding/binary"
	"flag"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/speaker"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("root", "", "original data directory")
	out := flag.String("out", "", "private PCM output file")
	flag.Parse()
	var containers []*assets.Container
	for _, name := range []string{"DATA2", "DATA3"} {
		read := func(ext string) []byte {
			b, e := os.ReadFile(filepath.Join(*root, name+ext))
			if e != nil {
				panic(e)
			}
			return b
		}
		c, e := assets.OpenContainer(read(".NAM"), read(".IDX"), read(".GRP"))
		if e != nil {
			panic(e)
		}
		containers = append(containers, c)
	}
	var pcm []byte
	for _, n := range []int{361, 499, 499} {
		var data []byte
		for _, c := range containers {
			if i, ok := c.ByName(speaker.VoiceName(n)); ok {
				data = c.Data(i)
				break
			}
		}
		if len(data) == 0 {
			panic("missing original clip")
		}
		for _, sample := range speaker.Render(speaker.NewClip(data), speaker.Rate(speaker.VoiceDivisor), 48000) {
			var frame [4]byte
			binary.LittleEndian.PutUint16(frame[:2], uint16(sample))
			binary.LittleEndian.PutUint16(frame[2:], uint16(sample))
			pcm = append(pcm, frame[:]...)
		}
	}
	f, e := os.OpenFile(*out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if e != nil {
		panic(e)
	}
	if _, e = f.Write(pcm); e != nil {
		panic(e)
	}
	if e = f.Close(); e != nil {
		panic(e)
	}
}
