package main

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	nurl "net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

func main() {
	url := flag.String("url", "http://storage.googleapis.com/vimeo-test/work-at-vimeo.mp4", "URL for download")
	threads := flag.Int("threads", 100, "Number of threads to download with")
	flag.Parse()
	u, err := nurl.Parse(*url)
	if err != nil {
		log.Fatal(err)
	}
	final_filename := strings.Replace(u.Path[1:], "/", "-", -1)
	defer timeTrack(time.Now(), "Full download")
	resp, err := http.Get(*url)
	if err != nil {
		log.Fatal(err)
		return
	}
	if len(resp.Header["Content-Length"]) < 1 {
		log.Fatal("No Content-Length Provided")
		return
	}
	content_size, _ := strconv.Atoi(resp.Header["Content-Length"][0])
	if resp.Header["Accept-Ranges"][0] == "bytes" {
		var wg sync.WaitGroup
		log.Println("Ranges Supported!")
		log.Println("Content Size:", resp.Header["Content-Length"][0])
		calculated_chunksize := int(content_size / *threads)
		log.Println("Chunk Size: ", int(calculated_chunksize))
		var end_byte int
		start_byte := 0
		chunks := 0
		for i := 0; i < *threads; i++ {
			filename := final_filename + ".part." + strconv.Itoa(i)
			wg.Add(1)
			//start_byte := i * int(calculated_chunksize)
			end_byte = start_byte + int(calculated_chunksize)
			log.Println("Dispatch bytes", start_byte, " to ", end_byte)
			go fetchChunk(int64(start_byte), int64(end_byte), *url, filename, &wg)
			start_byte = end_byte
			chunks++
		}
		if end_byte < content_size {
			wg.Add(1)
			start_byte = end_byte
			end_byte = content_size
			filename := final_filename + ".part." + strconv.Itoa(chunks)
			log.Println("Dispatch bytes", start_byte, " to ", end_byte)
			go fetchChunk(int64(start_byte), int64(end_byte), *url, filename, &wg)
			chunks++
		}
		wg.Wait()
		log.Println("Download Complete!")
		log.Println("Building File...")
		outfile, err := os.Create(final_filename)
		defer outfile.Close()
		if err != nil {
			log.Fatal(err)
			return
		}
		defer timeTrack(time.Now(), "File Assembled")
		for i := 0; i < chunks; i++ {
			filename := final_filename + ".part." + strconv.Itoa(i)
			assembleChunk(filename, outfile)
		}
		//Verify file size
		filestats, err := outfile.Stat()
		if err != nil {
			log.Fatal(err)
			return
		}
		actual_filesize := filestats.Size()
		if actual_filesize != int64(content_size) {
			log.Fatal("Actual Size: ", actual_filesize, "\nExpected: ", content_size)
			return
		}
		//Verify Md5
		if len(resp.Header["X-Goog-Hash"]) > 1 {
			content_md5, err := base64.StdEncoding.DecodeString(resp.Header["X-Goog-Hash"][1][4:])
			if err != nil {
				log.Fatal(err)
				return
			}
			if err != nil {
				log.Fatal(err)
				return
			}
			barray, _ := ioutil.ReadFile(final_filename)
			computed_hash := md5.Sum(barray)
			computed_slice := computed_hash[0:]
			if bytes.Compare(computed_slice, content_md5) != 0 {

				log.Fatal("WARNING: MD5 Sums don't match")
				return
			}
			log.Println("File MD5 Matches!")
		}
		log.Println("File Build Complete!")
		return
	}
	log.Println("Range Download unsupported")
	log.Println("Beginning full download...")
	fetchChunk(0, int64(content_size), *url, "no-range-"+final_filename, nil)
	log.Println("Download Complete")
}

func assembleChunk(filename string, outfile *os.File) {
	chunkFile, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer chunkFile.Close()
	io.Copy(outfile, chunkFile)
	os.Remove(filename)
}

func fetchChunk(start_byte, end_byte int64, url string, filename string, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	client := new(http.Client)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
		return
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start_byte, end_byte-1))
	res, err := client.Do(req)
	/*
		var retry int = 3
		var res *http.Response
		for i := retry; i > 0; i-- {
			res, err = client.Do(req)
			if res.StatusCode == 200 {
				retry = 3
				break
			}
			retry = i
		}
		if retry == 0 && res == nil {
			log.Fatal(err)
			return
		}
	*/
	if err != nil {
		log.Fatal(err)
		return
	}
	defer res.Body.Close()
	if err != nil {
		log.Fatal(err)
		return
	}
	outfile, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer outfile.Close()
	io.Copy(outfile, res.Body)
	log.Println("Finished Downloading byte ", start_byte)
	return
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}
