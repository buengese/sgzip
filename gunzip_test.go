// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgzip

import (
	"bytes"
	oldgz "compress/gzip"
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"io"
	"io/ioutil"
	prand "math/rand"
	"os"
	"strings"
	"testing"
	"time"

	kpgzip "github.com/klauspost/compress/gzip"
)

type gunzipTest struct {
	name string
	desc string
	raw  string
	gzip []byte
	meta GzipMetadata
	seek int64
	err  error
}

var emptyStream = gunzipTest{ // has 1 empty fixed-huffman block
	"empty.txt",
	"empty.txt",
	"",
	[]byte{
		0x1f, 0x8b, 0x08, 0x08, 0xf7, 0x5e, 0x14, 0x4a,
		0x00, 0x03, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e,
		0x74, 0x78, 0x74, 0x00, 0x03, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	},
	GzipMetadata{
		BlockSize: defaultBlockSize,
		Size:      0,
		BlockData: []uint32{
			20, 2,
		},
	},
	0,
	nil,
}

var seekingTests = []gunzipTest{
	{ // has 1 non-empty fixed huffman block
		"hello.txt",
		"hello.txt",
		"hello world\n",
		[]byte{
			0x1f, 0x8b, 0x08, 0x08, 0xc8, 0x58, 0x13, 0x4a,
			0x00, 0x03, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e,
			0x74, 0x78, 0x74, 0x00, 0xcb, 0x48, 0xcd, 0xc9,
			0xc9, 0x57, 0x28, 0xcf, 0x2f, 0xca, 0x49, 0xe1,
			0x02, 0x00, 0x2d, 0x3b, 0x08, 0xaf, 0x0c, 0x00,
			0x00, 0x00,
		},
		GzipMetadata{
			BlockSize: defaultBlockSize,
			Size:      12,
			BlockData: []uint32{
				20, 56,
			},
		},
		12,
		nil,
	},
	{ // has a fixed huffman block with some length-distance pairs
		"shesells.txt",
		"shesells.txt",
		"she sells seashells by the seashore\n",
		[]byte{
			0x1f, 0x8b, 0x08, 0x08, 0x72, 0x66, 0x8b, 0x4a,
			0x00, 0x03, 0x73, 0x68, 0x65, 0x73, 0x65, 0x6c,
			0x6c, 0x73, 0x2e, 0x74, 0x78, 0x74, 0x00, 0x2b,
			0xce, 0x48, 0x55, 0x28, 0x4e, 0xcd, 0xc9, 0x29,
			0x06, 0x92, 0x89, 0xc5, 0x19, 0x60, 0x56, 0x52,
			0xa5, 0x42, 0x09, 0x58, 0x18, 0x28, 0x90, 0x5f,
			0x94, 0xca, 0x05, 0x00, 0x76, 0xb0, 0x3b, 0xeb,
			0x24, 0x00, 0x00, 0x00,
		},
		GzipMetadata{
			BlockSize: defaultBlockSize,
			Size:      36,
			BlockData: []uint32{
				23, 29,
			},
		},
		17,
		nil,
	},
	{ // has dynamic huffman blocks
		"gettysburg",
		"gettysburg",
		"  Four score and seven years ago our fathers brought forth on\n" +
			"this continent, a new nation, conceived in Liberty, and dedicated\n" +
			"to the proposition that all men are created equal.\n" +
			"  Now we are engaged in a great Civil War, testing whether that\n" +
			"nation, or any nation so conceived and so dedicated, can long\n" +
			"endure.\n" +
			"  We are met on a great battle-field of that war.\n" +
			"  We have come to dedicate a portion of that field, as a final\n" +
			"resting place for those who here gave their lives that that\n" +
			"nation might live.  It is altogether fitting and proper that\n" +
			"we should do this.\n" +
			"  But, in a larger sense, we can not dedicate — we can not\n" +
			"consecrate — we can not hallow — this ground.\n" +
			"  The brave men, living and dead, who struggled here, have\n" +
			"consecrated it, far above our poor power to add or detract.\n" +
			"The world will little note, nor long remember what we say here,\n" +
			"but it can never forget what they did here.\n" +
			"  It is for us the living, rather, to be dedicated here to the\n" +
			"unfinished work which they who fought here have thus far so\n" +
			"nobly advanced.  It is rather for us to be here dedicated to\n" +
			"the great task remaining before us — that from these honored\n" +
			"dead we take increased devotion to that cause for which they\n" +
			"gave the last full measure of devotion —\n" +
			"  that we here highly resolve that these dead shall not have\n" +
			"died in vain — that this nation, under God, shall have a new\n" +
			"birth of freedom — and that government of the people, by the\n" +
			"people, for the people, shall not perish from this earth.\n" +
			"\n" +
			"Abraham Lincoln, November 19, 1863, Gettysburg, Pennsylvania\n",
		[]byte{
			0x1f, 0x8b, 0x08, 0x08, 0xd1, 0x12, 0x2b, 0x4a,
			0x00, 0x03, 0x67, 0x65, 0x74, 0x74, 0x79, 0x73,
			0x62, 0x75, 0x72, 0x67, 0x00, 0x65, 0x54, 0xcd,
			0x6e, 0xd4, 0x30, 0x10, 0xbe, 0xfb, 0x29, 0xe6,
			0x01, 0x42, 0xa5, 0x0a, 0x09, 0xc1, 0x11, 0x90,
			0x40, 0x48, 0xa8, 0xe2, 0x80, 0xd4, 0xf3, 0x24,
			0x9e, 0x24, 0x56, 0xbd, 0x9e, 0xc5, 0x76, 0x76,
			0x95, 0x1b, 0x0f, 0xc1, 0x13, 0xf2, 0x24, 0x7c,
			0x63, 0x77, 0x9b, 0x4a, 0x5c, 0xaa, 0x6e, 0x6c,
			0xcf, 0x7c, 0x7f, 0x33, 0x44, 0x5f, 0x74, 0xcb,
			0x54, 0x26, 0xcd, 0x42, 0x9c, 0x3c, 0x15, 0xb9,
			0x48, 0xa2, 0x5d, 0x38, 0x17, 0xe2, 0x45, 0xc9,
			0x4e, 0x67, 0xae, 0xab, 0xe0, 0xf7, 0x98, 0x75,
			0x5b, 0xd6, 0x4a, 0xb3, 0xe6, 0xba, 0x92, 0x26,
			0x57, 0xd7, 0x50, 0x68, 0xd2, 0x54, 0x43, 0x92,
			0x54, 0x07, 0x62, 0x4a, 0x72, 0xa5, 0xc4, 0x35,
			0x68, 0x1a, 0xec, 0x60, 0x92, 0x70, 0x11, 0x4f,
			0x21, 0xd1, 0xf7, 0x30, 0x4a, 0xae, 0xfb, 0xd0,
			0x9a, 0x78, 0xf1, 0x61, 0xe2, 0x2a, 0xde, 0x55,
			0x25, 0xd4, 0xa6, 0x73, 0xd6, 0xb3, 0x96, 0x60,
			0xef, 0xf0, 0x9b, 0x2b, 0x71, 0x8c, 0x74, 0x02,
			0x10, 0x06, 0xac, 0x29, 0x8b, 0xdd, 0x25, 0xf9,
			0xb5, 0x71, 0xbc, 0x73, 0x44, 0x0f, 0x7a, 0xa5,
			0xab, 0xb4, 0x33, 0x49, 0x0b, 0x2f, 0xbd, 0x03,
			0xd3, 0x62, 0x17, 0xe9, 0x73, 0xb8, 0x84, 0x48,
			0x8f, 0x9c, 0x07, 0xaa, 0x52, 0x00, 0x6d, 0xa1,
			0xeb, 0x2a, 0xc6, 0xa0, 0x95, 0x76, 0x37, 0x78,
			0x9a, 0x81, 0x65, 0x7f, 0x46, 0x4b, 0x45, 0x5f,
			0xe1, 0x6d, 0x42, 0xe8, 0x01, 0x13, 0x5c, 0x38,
			0x51, 0xd4, 0xb4, 0x38, 0x49, 0x7e, 0xcb, 0x62,
			0x28, 0x1e, 0x3b, 0x82, 0x93, 0x54, 0x48, 0xf1,
			0xd2, 0x7d, 0xe4, 0x5a, 0xa3, 0xbc, 0x99, 0x83,
			0x44, 0x4f, 0x3a, 0x77, 0x36, 0x57, 0xce, 0xcf,
			0x2f, 0x56, 0xbe, 0x80, 0x90, 0x9e, 0x84, 0xea,
			0x51, 0x1f, 0x8f, 0xcf, 0x90, 0xd4, 0x60, 0xdc,
			0x5e, 0xb4, 0xf7, 0x10, 0x0b, 0x26, 0xe0, 0xff,
			0xc4, 0xd1, 0xe5, 0x67, 0x2e, 0xe7, 0xc8, 0x93,
			0x98, 0x05, 0xb8, 0xa8, 0x45, 0xc0, 0x4d, 0x09,
			0xdc, 0x84, 0x16, 0x2b, 0x0d, 0x9a, 0x21, 0x53,
			0x04, 0x8b, 0xd2, 0x0b, 0xbd, 0xa2, 0x4c, 0xa7,
			0x60, 0xee, 0xd9, 0xe1, 0x1d, 0xd1, 0xb7, 0x4a,
			0x30, 0x8f, 0x63, 0xd5, 0xa5, 0x8b, 0x33, 0x87,
			0xda, 0x1a, 0x18, 0x79, 0xf3, 0xe3, 0xa6, 0x17,
			0x94, 0x2e, 0xab, 0x6e, 0xa0, 0xe3, 0xcd, 0xac,
			0x50, 0x8c, 0xca, 0xa7, 0x0d, 0x76, 0x37, 0xd1,
			0x23, 0xe7, 0x05, 0x57, 0x8b, 0xa4, 0x22, 0x83,
			0xd9, 0x62, 0x52, 0x25, 0xad, 0x07, 0xbb, 0xbf,
			0xbf, 0xff, 0xbc, 0xfa, 0xee, 0x20, 0x73, 0x91,
			0x29, 0xff, 0x7f, 0x02, 0x71, 0x62, 0x84, 0xb5,
			0xf6, 0xb5, 0x25, 0x6b, 0x41, 0xde, 0x92, 0xb7,
			0x76, 0x3f, 0x91, 0x91, 0x31, 0x1b, 0x41, 0x84,
			0x62, 0x30, 0x0a, 0x37, 0xa4, 0x5e, 0x18, 0x3a,
			0x99, 0x08, 0xa5, 0xe6, 0x6d, 0x59, 0x22, 0xec,
			0x33, 0x39, 0x86, 0x26, 0xf5, 0xab, 0x66, 0xc8,
			0x08, 0x20, 0xcf, 0x0c, 0xd7, 0x47, 0x45, 0x21,
			0x0b, 0xf6, 0x59, 0xd5, 0xfe, 0x5c, 0x8d, 0xaa,
			0x12, 0x7b, 0x6f, 0xa1, 0xf0, 0x52, 0x33, 0x4f,
			0xf5, 0xce, 0x59, 0xd3, 0xab, 0x66, 0x10, 0xbf,
			0x06, 0xc4, 0x31, 0x06, 0x73, 0xd6, 0x80, 0xa2,
			0x78, 0xc2, 0x45, 0xcb, 0x03, 0x65, 0x39, 0xc9,
			0x09, 0xd1, 0x06, 0x04, 0x33, 0x1a, 0x5a, 0xf1,
			0xde, 0x01, 0xb8, 0x71, 0x83, 0xc4, 0xb5, 0xb3,
			0xc3, 0x54, 0x65, 0x33, 0x0d, 0x5a, 0xf7, 0x9b,
			0x90, 0x7c, 0x27, 0x1f, 0x3a, 0x58, 0xa3, 0xd8,
			0xfd, 0x30, 0x5f, 0xb7, 0xd2, 0x66, 0xa2, 0x93,
			0x1c, 0x28, 0xb7, 0xe9, 0x1b, 0x0c, 0xe1, 0x28,
			0x47, 0x26, 0xbb, 0xe9, 0x7d, 0x7e, 0xdc, 0x96,
			0x10, 0x92, 0x50, 0x56, 0x7c, 0x06, 0xe2, 0x27,
			0xb4, 0x08, 0xd3, 0xda, 0x7b, 0x98, 0x34, 0x73,
			0x9f, 0xdb, 0xf6, 0x62, 0xed, 0x31, 0x41, 0x13,
			0xd3, 0xa2, 0xa8, 0x4b, 0x3a, 0xc6, 0x1d, 0xe4,
			0x2f, 0x8c, 0xf8, 0xfb, 0x97, 0x64, 0xf4, 0xb6,
			0x2f, 0x80, 0x5a, 0xf3, 0x56, 0xe0, 0x40, 0x50,
			0xd5, 0x19, 0xd0, 0x1e, 0xfc, 0xca, 0xe5, 0xc9,
			0xd4, 0x60, 0x00, 0x81, 0x2e, 0xa3, 0xcc, 0xb6,
			0x52, 0xf0, 0xb4, 0xdb, 0x69, 0x99, 0xce, 0x7a,
			0x32, 0x4c, 0x08, 0xed, 0xaa, 0x10, 0x10, 0xe3,
			0x6f, 0xee, 0x99, 0x68, 0x95, 0x9f, 0x04, 0x71,
			0xb2, 0x49, 0x2f, 0x62, 0xa6, 0x5e, 0xb4, 0xef,
			0x02, 0xed, 0x4f, 0x27, 0xde, 0x4a, 0x0f, 0xfd,
			0xc1, 0xcc, 0xdd, 0x02, 0x8f, 0x08, 0x16, 0x54,
			0xdf, 0xda, 0xca, 0xe0, 0x82, 0xf1, 0xb4, 0x31,
			0x7a, 0xa9, 0x81, 0xfe, 0x90, 0xb7, 0x3e, 0xdb,
			0xd3, 0x35, 0xc0, 0x20, 0x80, 0x33, 0x46, 0x4a,
			0x63, 0xab, 0xd1, 0x0d, 0x29, 0xd2, 0xe2, 0x84,
			0xb8, 0xdb, 0xfa, 0xe9, 0x89, 0x44, 0x86, 0x7c,
			0xe8, 0x0b, 0xe6, 0x02, 0x6a, 0x07, 0x9b, 0x96,
			0xd0, 0xdb, 0x2e, 0x41, 0x4c, 0xa1, 0xd5, 0x57,
			0x45, 0x14, 0xfb, 0xe3, 0xa6, 0x72, 0x5b, 0x87,
			0x6e, 0x0c, 0x6d, 0x5b, 0xce, 0xe0, 0x2f, 0xe2,
			0x21, 0x81, 0x95, 0xb0, 0xe8, 0xb6, 0x32, 0x0b,
			0xb2, 0x98, 0x13, 0x52, 0x5d, 0xfb, 0xec, 0x63,
			0x17, 0x8a, 0x9e, 0x23, 0x22, 0x36, 0xee, 0xcd,
			0xda, 0xdb, 0xcf, 0x3e, 0xf1, 0xc7, 0xf1, 0x01,
			0x12, 0x93, 0x0a, 0xeb, 0x6f, 0xf2, 0x02, 0x15,
			0x96, 0x77, 0x5d, 0xef, 0x9c, 0xfb, 0x88, 0x91,
			0x59, 0xf9, 0x84, 0xdd, 0x9b, 0x26, 0x8d, 0x80,
			0xf9, 0x80, 0x66, 0x2d, 0xac, 0xf7, 0x1f, 0x06,
			0xba, 0x7f, 0xff, 0xee, 0xed, 0x40, 0x5f, 0xa5,
			0xd6, 0xbd, 0x8c, 0x5b, 0x46, 0xd2, 0x7e, 0x48,
			0x4a, 0x65, 0x8f, 0x08, 0x42, 0x60, 0xf7, 0x0f,
			0xb9, 0x16, 0x0b, 0x0c, 0x1a, 0x06, 0x00, 0x00,
		},
		GzipMetadata{
			BlockSize: defaultBlockSize,
			Size:      1562,
			BlockData: []uint32{
				21, 787,
			},
		},
		721,
		nil,
	},
}

var errTests = []gunzipTest{
	{ // concatenation
		"hello.txt",
		"hello.txt x2",
		"hello world\n" +
			"hello world\n",
		[]byte{
			0x1f, 0x8b, 0x08, 0x08, 0xc8, 0x58, 0x13, 0x4a,
			0x00, 0x03, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e,
			0x74, 0x78, 0x74, 0x00, 0xcb, 0x48, 0xcd, 0xc9,
			0xc9, 0x57, 0x28, 0xcf, 0x2f, 0xca, 0x49, 0xe1,
			0x02, 0x00, 0x2d, 0x3b, 0x08, 0xaf, 0x0c, 0x00,
			0x00, 0x00,
			0x1f, 0x8b, 0x08, 0x08, 0xc8, 0x58, 0x13, 0x4a,
			0x00, 0x03, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e,
			0x74, 0x78, 0x74, 0x00, 0xcb, 0x48, 0xcd, 0xc9,
			0xc9, 0x57, 0x28, 0xcf, 0x2f, 0xca, 0x49, 0xe1,
			0x02, 0x00, 0x2d, 0x3b, 0x08, 0xaf, 0x0c, 0x00,
			0x00, 0x00,
		},
		GzipMetadata{},
		0,
		nil,
	},
	{ // has 1 non-empty fixed huffman block then garbage
		"hello.txt",
		"hello.txt + garbage",
		"hello world\n",
		[]byte{
			0x1f, 0x8b, 0x08, 0x08, 0xc8, 0x58, 0x13, 0x4a,
			0x00, 0x03, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e,
			0x74, 0x78, 0x74, 0x00, 0xcb, 0x48, 0xcd, 0xc9,
			0xc9, 0x57, 0x28, 0xcf, 0x2f, 0xca, 0x49, 0xe1,
			0x02, 0x00, 0x2d, 0x3b, 0x08, 0xaf, 0x0c, 0x00,
			0x00, 0x00, 'g', 'a', 'r', 'b', 'a', 'g', 'e', '!', '!', '!',
		},
		GzipMetadata{},
		0,
		ErrHeader,
	},
	{ // has 1 non-empty fixed huffman block not enough header
		"hello.txt",
		"hello.txt + garbage",
		"hello world\n",
		[]byte{
			0x1f, 0x8b, 0x08, 0x08, 0xc8, 0x58, 0x13, 0x4a,
			0x00, 0x03, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e,
			0x74, 0x78, 0x74, 0x00, 0xcb, 0x48, 0xcd, 0xc9,
			0xc9, 0x57, 0x28, 0xcf, 0x2f, 0xca, 0x49, 0xe1,
			0x02, 0x00, 0x2d, 0x3b, 0x08, 0xaf, 0x0c, 0x00,
			0x00, 0x00, gzipID1,
		},
		GzipMetadata{},
		0,
		io.ErrUnexpectedEOF,
	},
	{ // has 1 non-empty fixed huffman block but corrupt checksum
		"hello.txt",
		"hello.txt + corrupt checksum",
		"hello world\n",
		[]byte{
			0x1f, 0x8b, 0x08, 0x08, 0xc8, 0x58, 0x13, 0x4a,
			0x00, 0x03, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e,
			0x74, 0x78, 0x74, 0x00, 0xcb, 0x48, 0xcd, 0xc9,
			0xc9, 0x57, 0x28, 0xcf, 0x2f, 0xca, 0x49, 0xe1,
			0x02, 0x00, 0xff, 0xff, 0xff, 0xff, 0x0c, 0x00,
			0x00, 0x00,
		},
		GzipMetadata{},
		0,
		ErrChecksum,
	},
	{ // has 1 non-empty fixed huffman block but corrupt size
		"hello.txt",
		"hello.txt + corrupt size",
		"hello world\n",
		[]byte{
			0x1f, 0x8b, 0x08, 0x08, 0xc8, 0x58, 0x13, 0x4a,
			0x00, 0x03, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x2e,
			0x74, 0x78, 0x74, 0x00, 0xcb, 0x48, 0xcd, 0xc9,
			0xc9, 0x57, 0x28, 0xcf, 0x2f, 0xca, 0x49, 0xe1,
			0x02, 0x00, 0x2d, 0x3b, 0x08, 0xaf, 0xff, 0x00,
			0x00, 0x00,
		},
		GzipMetadata{},
		0,
		ErrChecksum,
	},
}

var gunzipTests = append(append([]gunzipTest{emptyStream}, seekingTests...), errTests...)

func TestDecompressor(t *testing.T) {
	b := new(bytes.Buffer)
	for _, tt := range gunzipTests {
		in := bytes.NewReader(tt.gzip)
		gzip, err := NewReader(in)
		if err != nil {
			t.Errorf("%s: NewReader: %v", tt.name, err)
			continue
		}
		defer gzip.Close()
		if tt.name != gzip.Name {
			t.Errorf("%s: got name %s", tt.name, gzip.Name)
		}
		b.Reset()
		n, err := io.Copy(b, gzip)
		if err != tt.err {
			t.Errorf("%s: io.Copy: %v want %v", tt.name, err, tt.err)
		}
		s := b.String()
		if s != tt.raw {
			t.Errorf("%s: got %d-byte %q want %d-byte %q", tt.name, n, s, len(tt.raw), tt.raw)
		}

		// Test Reader Reset.
		in = bytes.NewReader(tt.gzip)
		err = gzip.Reset(in)
		if err != nil {
			t.Errorf("%s: Reset: %v", tt.name, err)
			continue
		}
		if tt.name != gzip.Name {
			t.Errorf("%s: got name %v", tt.name, gzip.Name)
		}
		b.Reset()
		n, err = io.Copy(b, gzip)
		if err != tt.err {
			t.Errorf("%s: io.Copy: %v want %v", tt.name, err, tt.err)
		}
		s = b.String()
		if s != tt.raw {
			t.Errorf("%s: got %d-byte %q want %d-byte %q", tt.name, n, s, len(tt.raw), tt.raw)
		}
	}
}

func TestSeekUnseekable(t *testing.T) {
	in := bytes.NewReader(emptyStream.gzip)
	gzip, err := NewReader(in)
	if err != nil {
		t.Errorf("%s: NewReader: %v", emptyStream.name, err)
	}
	_, err = gzip.Seek(100000, io.SeekStart)
	if err != ErrUnsupported {
		t.Errorf("%s: gzip.Seek: %v want %v", emptyStream.name, err, ErrUnsupported)
	}
	gzip.Close()
}

func TestInvalidSeek(t *testing.T) {
	in := bytes.NewReader(emptyStream.gzip)
	gzip, err := NewSeekingReader(in, &emptyStream.meta)
	if err != nil {
		t.Errorf("%s: NewReader: %v", emptyStream.name, err)
	}
	_, err = gzip.Seek(100000, io.SeekStart)
	if err != ErrInvalidSeek {
		t.Errorf("%s: gzip.Seek: %v want %v", emptyStream.name, err, ErrInvalidSeek)
	}
	gzip.Close()
}

func TestDecompressorWithSeek(t *testing.T) {
	b := new(bytes.Buffer)
	for _, tt := range seekingTests {
		fmt.Printf("Name: %s\n", tt.name)
		in := bytes.NewReader(tt.gzip)
		gzip, err := NewSeekingReader(in, &tt.meta)
		if err != nil {
			t.Errorf("%s: NewReader: %v", tt.name, err)
			continue
		}
		defer gzip.Close()
		if tt.name != gzip.Name {
			t.Errorf("%s: got name %s", tt.name, gzip.Name)
		}
		b.Reset()
		_, err = gzip.Seek(tt.seek, io.SeekStart)
		if err != nil {
			t.Errorf("%s: gzip.Seek error %v", tt.name, err)
			continue
		}
		n, err := io.Copy(b, gzip)
		if err != tt.err {
			t.Errorf("%s: io.Copy: %v want %v", tt.name, err, tt.err)
		}

		s := b.Bytes()
		raw := []byte(tt.raw)[tt.seek:]
		if !bytes.Equal(s, raw) {
			t.Errorf("%s: got %d-byte %x want %d-byte %x", tt.name, n, s, len(raw), raw)
		}
	}
}

func TestReaderAt(t *testing.T) {
	b := new(bytes.Buffer)
	tt := seekingTests[2]
	gzip, err := NewReaderAt(bytes.NewReader(tt.gzip), &tt.meta, tt.seek)
	if err != nil {
		t.Fatalf("NewReaderAt: %v", err)
	}
	defer gzip.Close()
	n, err := io.Copy(b, gzip)
	if err != nil {
		t.Errorf("io.Copy: %v", err)
	}
	s := b.Bytes()
	raw := []byte(tt.raw)[tt.seek:]
	if !bytes.Equal(s, raw) {
		t.Errorf("%s: got %d-byte want %d-byte", tt.name, n, len(raw))
	}
}

func TestDecompressFileWithSeek(t *testing.T) {
	f, err := os.Open("testdata/test.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	mf, err := os.Open("testdata/test.json.dat")
	if err != nil {
		t.Fatal(err)
	}
	defer mf.Close()
	of, err := os.Open("testdata/test.json")
	if err != nil {
		t.Fatal(err)
	}
	defer of.Close()
	var meta GzipMetadata
	err = gob.NewDecoder(mf).Decode(&meta)
	if err != nil {
		t.Fatalf("Invalid metadata %s", err)
	}

	gzip, err := NewSeekingReader(f, &meta)
	if err != nil {
		t.Fatalf("NewReader(testdata/test.json.gz): %v", err)
	}
	defer gzip.Close()

	if _, err = gzip.Seek(89179, io.SeekStart); err != nil {
		t.Errorf("gzip.Seek error %v", err)
	}
	if _, err = of.Seek(89179, io.SeekStart); err != nil {
		t.Errorf("of.Seek error %v", err)
	}

	var b1 = new(bytes.Buffer)
	if _, err = io.Copy(b1, gzip); err != nil {
		t.Errorf("gzip: io.Copy: %v", err)
	}
	var b2 = new(bytes.Buffer)
	if _, err = io.Copy(b2, of); err != nil {
		t.Errorf("of: io.Copy: %v", err)
	}
	if !bytes.Equal(b1.Bytes(), b2.Bytes()) {
		t.Errorf("Seek did not match original file")
	}
}

func TestMultiSeek(t *testing.T) {
	f, err := os.Open("testdata/test.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	mf, err := os.Open("testdata/test.json.dat")
	if err != nil {
		t.Fatal(err)
	}
	defer mf.Close()
	of, err := os.Open("testdata/test.json")
	if err != nil {
		t.Fatal(err)
	}
	defer of.Close()
	var meta GzipMetadata
	err = gob.NewDecoder(mf).Decode(&meta)
	if err != nil {
		t.Fatalf("Invalid metadata %s", err)
	}

	gzip, err := NewSeekingReader(f, &meta)
	if err != nil {
		t.Fatalf("NewReader(testdata/test.json.gz): %v", err)
	}
	defer gzip.Close()

	prand.Seed(1337)
	var buf1 = make([]byte, 256)
	var buf2 = make([]byte, 256)
	for i := 0; i < 10; i++ {
		pos := prand.Intn(147154)
		_, err = gzip.Seek(int64(pos), io.SeekStart)
		if err != nil {
			t.Errorf("gzip.Seek error %v", err)
		}
		_, err = of.Seek(int64(pos), io.SeekStart)
		if err != nil {
			t.Errorf("of.Seek error %v", err)
		}

		_, err = gzip.Read(buf1)
		if err != nil {
			t.Errorf("gzip.Read error %v", err)
		}
		_, err = of.Read(buf2)
		if err != nil {
			t.Errorf("of.Read error %v", err)
		}
		if !bytes.Equal(buf1, buf2) {
			t.Errorf("read does not match original file.")
		}

		// lets read another buffer just to make sure
		_, err = gzip.Read(buf1)
		if err != nil {
			t.Errorf("gzip.Read error %v", err)
		}
		_, err = of.Read(buf2)
		if err != nil {
			t.Errorf("of.Read error %v", err)
		}
		if !bytes.Equal(buf1, buf2) {
			t.Errorf("read does not match original file.")
		}
	}
}

func TestIssue6550(t *testing.T) {
	f, err := os.Open("testdata/issue6550.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gzip, err := NewReader(f)
	if err != nil {
		t.Fatalf("NewReader(testdata/issue6550.gz): %v", err)
	}
	defer gzip.Close()
	done := make(chan bool, 1)
	go func() {
		_, err := io.Copy(ioutil.Discard, gzip)
		if err == nil {
			t.Errorf("Copy succeeded")
		} else {
			t.Logf("Copy failed (correctly): %v", err)
		}
		done <- true
	}()
	select {
	case <-time.After(1 * time.Second):
		t.Errorf("Copy hung")
	case <-done:
		// ok
	}
}

func TestInitialReset(t *testing.T) {
	var r Reader
	if err := r.Reset(bytes.NewReader(seekingTests[1].gzip)); err != nil {
		t.Error(err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, &r); err != nil {
		t.Error(err)
	}
	if s := buf.String(); s != seekingTests[1].raw {
		t.Errorf("got %q want %q", s, gunzipTests[1].raw)
	}
}

func TestMultiReset(t *testing.T) {
	var r Reader
	for i := 0; i < 10; i++ {
		if err := r.Reset(bytes.NewReader(seekingTests[1].gzip)); err != nil {
			t.Error(err)
		}
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, &r); err != nil {
		t.Error(err)
	}
	if s := buf.String(); s != seekingTests[1].raw {
		t.Errorf("got %q want %q", s, seekingTests[1].gzip)
	}
}

func TestResetEOF(t *testing.T) {
	in := bytes.NewReader(seekingTests[1].gzip)
	r, err := NewReader(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		t.Fatal(err)
	}
	// reset reader with same bytes.Reader that is now EOF
	if err := r.Reset(in); err != io.EOF {
		t.Errorf("reset: err=%v, want io.EOF", err)
	}
	// try reading something
	if _, err := io.Copy(ioutil.Discard, r); err != io.EOF {
		t.Errorf("io.Copy: err=%v, want io.EOF", err)
	}
}

func TestMultistreamFalse(t *testing.T) {
	// Find concatenation test.
	var tt gunzipTest
	for _, tt = range gunzipTests {
		if strings.HasSuffix(tt.desc, " x2") {
			goto Found
		}
	}
	t.Fatal("cannot find hello.txt x2 in gunzip tests")

Found:
	br := bytes.NewReader(tt.gzip)
	var r Reader
	if err := r.Reset(br); err != nil {
		t.Fatalf("first reset: %v", err)
	}

	// Expect two streams with "hello world\n", then real EOF.
	const hello = "hello world\n"

	r.Multistream(false)
	data, err := ioutil.ReadAll(&r)
	if string(data) != hello || err != nil {
		t.Fatalf("first stream = %q, %v, want %q, %v", string(data), err, hello, nil)
	}

	if err := r.Reset(br); err != nil {
		t.Fatalf("second reset: %v", err)
	}
	r.Multistream(false)
	data, err = ioutil.ReadAll(&r)
	if string(data) != hello || err != nil {
		t.Fatalf("second stream = %q, %v, want %q, %v", string(data), err, hello, nil)
	}

	if err := r.Reset(br); err != io.EOF {
		t.Fatalf("third reset: err=%v, want io.EOF", err)
	}
}

func TestWriteTo(t *testing.T) {
	input := make([]byte, 100000)
	n, err := rand.Read(input)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(input) {
		t.Fatal("did not fill buffer")
	}
	compressed := &bytes.Buffer{}
	// Do it twice to test MultiStream functionality
	for i := 0; i < 2; i++ {
		w, err := NewWriterLevel(compressed, -2)
		if err != nil {
			t.Fatal(err)
		}
		n, err = w.Write(input)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(input) {
			t.Fatal("did not fill buffer")
		}
		w.Close()
	}
	input = append(input, input...)
	buf := compressed.Bytes()

	dec, err := NewReader(bytes.NewBuffer(buf))
	if err != nil {
		t.Fatal(err)
	}
	// ReadAll does not use WriteTo, but we wrap it in a NopCloser to be sure.
	readall, err := ioutil.ReadAll(ioutil.NopCloser(dec))
	if err != nil {
		t.Fatal(err)
	}
	if len(readall) != len(input) {
		t.Fatal("did not decompress everything")
	}
	if bytes.Compare(readall, input) != 0 {
		t.Fatal("output did not match input")
	}

	dec, err = NewReader(bytes.NewBuffer(buf))
	if err != nil {
		t.Fatal(err)
	}
	wtbuf := &bytes.Buffer{}
	written, err := dec.WriteTo(wtbuf)
	if err != nil {
		t.Fatal(err)
	}
	if written != int64(len(input)) {
		t.Error("Returned length did not match, expected", len(input), "got", written)
	}
	if wtbuf.Len() != len(input) {
		t.Error("Actual Length did not match, expected", len(input), "got", wtbuf.Len())
	}
	if bytes.Compare(wtbuf.Bytes(), input) != 0 {
		t.Fatal("output did not match input")
	}
}

func BenchmarkGunzipCopy(b *testing.B) {
	dat, _ := ioutil.ReadFile("testdata/test.json")
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dst := &bytes.Buffer{}
	w, _ := NewWriterLevel(dst, 1)
	_, err := w.Write(dat)
	if err != nil {
		b.Fatal(err)
	}
	w.Close()
	input := dst.Bytes()
	r, err := NewReader(bytes.NewBuffer(input))
	b.SetBytes(int64(len(dat)))
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err = r.Reset(bytes.NewBuffer(input))
		if err != nil {
			b.Fatal(err)
		}
		_, err = io.Copy(ioutil.Discard, r)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGunzipReadAll(b *testing.B) {
	dat, _ := ioutil.ReadFile("testdata/test.json")
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dst := &bytes.Buffer{}
	w, _ := NewWriterLevel(dst, 1)
	_, err := w.Write(dat)
	if err != nil {
		b.Fatal(err)
	}
	w.Close()
	input := dst.Bytes()
	r, err := NewReader(bytes.NewBuffer(input))
	b.SetBytes(int64(len(dat)))
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err = r.Reset(bytes.NewBuffer(input))
		if err != nil {
			b.Fatal(err)
		}
		_, err = ioutil.ReadAll(ioutil.NopCloser(r))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGunzipStdLib(b *testing.B) {
	dat, _ := ioutil.ReadFile("testdata/test.json")
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dst := &bytes.Buffer{}
	w, _ := NewWriterLevel(dst, 1)
	_, err := w.Write(dat)
	if err != nil {
		b.Fatal(err)
	}
	w.Close()
	input := dst.Bytes()
	r, err := oldgz.NewReader(bytes.NewBuffer(input))
	b.SetBytes(int64(len(dat)))
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err = r.Reset(bytes.NewBuffer(input))
		if err != nil {
			b.Fatal(err)
		}
		_, err = io.Copy(ioutil.Discard, r)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGunzipFlate(b *testing.B) {
	dat, _ := ioutil.ReadFile("testdata/test.json")
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dat = append(dat, dat...)
	dst := &bytes.Buffer{}
	w, _ := NewWriterLevel(dst, 1)
	_, err := w.Write(dat)
	if err != nil {
		b.Fatal(err)
	}
	w.Close()
	input := dst.Bytes()
	r, err := kpgzip.NewReader(bytes.NewBuffer(input))
	b.SetBytes(int64(len(dat)))
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err = r.Reset(bytes.NewBuffer(input))
		if err != nil {
			b.Fatal(err)
		}
		_, err = io.Copy(ioutil.Discard, r)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestTruncatedGunzip(t *testing.T) {
	in := []byte(strings.Repeat("ASDFASDFASDFASDFASDF", 1000))
	var buf bytes.Buffer
	enc := kpgzip.NewWriter(&buf)
	_, err := enc.Write(in)
	if err != nil {
		t.Fatal(err)
	}
	enc.Close()
	testdata := buf.Bytes()
	for i := 5; i < len(testdata); i += 10 {
		timer := time.NewTimer(time.Second)
		done := make(chan struct{})
		fail := make(chan struct{})
		go func() {
			r, err := NewReader(bytes.NewBuffer(testdata[:i]))
			if err == nil {
				b, err := ioutil.ReadAll(r)
				if err == nil && !bytes.Equal(testdata[:i], b) {
					close(fail)
				}
			}
			close(done)
		}()
		select {
		case <-timer.C:
			t.Fatal("Timeout decoding")
		case <-fail:
			t.Fatal("No error, but mismatch")
		case <-done:
			timer.Stop()
		}
	}
}

func TestTruncatedGunzipBlocks(t *testing.T) {
	var in = make([]byte, 512*10)
	rand.Read(in)
	var buf bytes.Buffer
	for i := 0; i < len(in); i += 512 {
		enc, _ := kpgzip.NewWriterLevel(&buf, 0)
		_, err := enc.Write(in[:i])
		if err != nil {
			t.Fatal(err)
		}
		enc.Close()

		timer := time.NewTimer(time.Second)
		done := make(chan struct{})
		fail := make(chan struct{})
		go func() {
			r, err := NewReaderN(&buf, 512, 10)
			if err == nil {
				b, err := ioutil.ReadAll(r)
				if err == nil && !bytes.Equal(b, in[:i]) {
					close(fail)
				}
			}
			close(done)
		}()
		select {
		case <-timer.C:
			t.Fatal("Timeout decoding")
		case <-fail:
			t.Fatal("No error, but mismatch")
		case <-done:
			timer.Stop()
		}
	}
}
