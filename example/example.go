// +build !windows

package main

import (
	"fmt"
	"os"
	"flag"
	"strconv"
	"strings"
	"github.com/r0t3n/ext2fs"
)

type ulist []uint64

func (l *ulist) String() string {
   return fmt.Sprintf("%v", *l)
}

func (l *ulist) Set(value string) error {
	v, err := strconv.ParseUint(value, 10, 64)
    if err != nil {
		return err
	}
	if v < 0 {
		return fmt.Errorf("Invalid block value '%d'", v)
	}
	*l = append(*l, v)
	return nil
}

var blocks ulist
var inodes ulist

func main() {
	var device string
	var noheader bool
	var stats bool
	var err error

	flag.StringVar(&device, "device", "", "Path to EXT4 filesystem")
	flag.Var(&blocks, "block", "List of blocks to lookup")
	flag.Var(&inodes, "inode", "List of inodes to lookup")
	flag.BoolVar(&noheader, "H", false, "Do not print header")
	flag.BoolVar(&stats, "s", false, "Print statistics")
	flag.Parse()

	if device == "" || len(blocks) + len(inodes) <= 0 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	fs := ext2fs.New()

	if err = fs.Open(device, 0, 0, 0); err != nil {
		fmt.Printf("Error opening filesystem %s: %v", device, err)
		os.Exit(1)
	}

	var fsobjs []ext2fs.FilesystemObject
	var stat ext2fs.Stats
	if fsobjs, stat, err = fs.LookupFilesystemObjects(inodes, blocks); err != nil {
		fmt.Printf("Error looking up filesystem object(s): %v", err)
		fmt.Println()
		if err = fs.Close(); err != nil {
			fmt.Printf("Error closing filesystem %s: %v", device, err)
			fmt.Println()
		}
		os.Exit(1)
	}
	if stats {
		fmt.Printf("Processed %d dirs, %d files, %d matches", stat.Dirs, stat.Files, len(fsobjs))
		fmt.Println()
	}
	if !noheader {
		fmt.Println("Inode	Filename	[flags]	#Blocks	Blocks")
	}

	for _, fsobj := range fsobjs {
		fmt.Printf("%d	%s", fsobj.Inode, fsobj.Path)

		var flags []string
		if fsobj.Type == ext2fs.EXT2_DIR_BLOCK {
			flags = append(flags, "D")
		} else if fsobj.Type == ext2fs.EXT2_EXTENT_BLOCK {
			flags = append(flags, "e")
		} else if fsobj.Type == ext2fs.EXT2_DATA_BLOCK {
			flags = append(flags, "d")
		}
		fmt.Printf("	[%s]", strings.Join(flags, ","))

		var count uint64
		if len(fsobj.Blocks) > 0 {
			var str []string
			for _, block := range fsobj.Blocks {
				if block.Start == block.End {
					count++
				} else {
					count += uint64(block.End) - uint64(block.Start)
				}
				str = append(str, fmt.Sprintf("%d-%d", block.Start, block.End))
			}
			fmt.Printf("#%d	%s", count, strings.Join(str, " "))
		}

		fmt.Println()
	}

	if err = fs.Close(); err != nil {
		fmt.Printf("Error closing filesystem %s: %v", device, err)
		fmt.Println()
		os.Exit(1)
	}
}