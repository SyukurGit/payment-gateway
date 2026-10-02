package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func main() {
	f, err := os.Open("qris-shopee.jpeg")
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		fmt.Printf("Error decoding image (%s): %v\n", format, err)
		return
	}

	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		fmt.Printf("Error creating bitmap: %v\n", err)
		return
	}

	reader := qrcode.NewQRCodeReader()
	res, err := reader.Decode(bmp, nil)
	if err != nil {
		fmt.Printf("Error decoding QR: %v\n", err)
		return
	}

	fmt.Println("=== DECODED QRIS STRING ===")
	fmt.Println(res.GetText())
}
