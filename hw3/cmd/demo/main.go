package main

import (
	"crypto/sha256"
	"downloader/internal"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

var resources = map[string]string{
	"https://download.fedoraproject.org/pub/fedora/linux/releases/test/44_Beta/KDE/x86_64/iso/Fedora-KDE-Desktop-Live-44_Beta-1.2.x86_64.iso": "7608815abd264c6f26b606bbd919a50a6950fee61a839b6f8d5891cca74a365a",
	//"https://nbg1-speed.hetzner.com/100MB.bin": "20492a4d0d84f8beb1767f6616229f85d44c2827b64bdbfb260ee12fa1109e0e",
}

func verifyChecksum(trueChecksum string, file string) bool {
	if len(trueChecksum) == 0 {
		return true
	}

	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}

	computedChecksum := fmt.Sprintf("%x", h.Sum(nil))
	fmt.Println(computedChecksum)
	return trueChecksum == computedChecksum
}

func main() {
	downloader := internal.NewDownloader()
	for resource, sum := range resources {
		i := strings.LastIndex(resource, "/")
		var filename string
		if i > 0 {
			filename = resource[i+1:]
		} else {
			filename = resource
		}

		err := downloader.Download(resource, "/tmp/"+filename)
		if err != nil {
			fmt.Println(err.Error())
		}

		fmt.Println("Checksums equal: ", verifyChecksum(sum, "/tmp/"+filename))
	}
}
