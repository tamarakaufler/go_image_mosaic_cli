// The application changes the provided image into a mosaic of images
//		* concurrent implementation with goroutines
//		* uses average tile pixel value
// Usage:
//			./go_image_mosaic -i image_path -t 10
package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"math"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/nfnt/resize"
)

type Tile interface {
	getTileColours()
}

type TileImage struct {
	scaled       image.Image
	averagePixel color.Color
	cornerPixel  color.Color
}

func (tile *TileImage) getTileColours(mutex *sync.Mutex, imagePattern *regexp.Regexp,
	imageDir string, filename string,
	tiles map[string]*TileImage, tileImages map[string]color.Color) {

	if !imagePattern.MatchString(filename) {
		return
	}

	file, err := os.Open(imageDir + filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// encode into an image
	tileImage, err := jpeg.Decode(bufio.NewReader(file))
	if err != nil {
		//log.Printf(">>> %v", err)
		return
	}

	xMin := tileImage.Bounds().Min.X
	xMax := tileImage.Bounds().Max.X
	yMin := tileImage.Bounds().Min.Y
	yMax := tileImage.Bounds().Max.Y

	var rSum, gSum, bSum uint32 = 0, 0, 0

	for y := yMin; y <= yMax; y++ {
		for x := xMin; x <= xMax; x++ {

			r, g, b, _ := tileImage.At(x, y).RGBA()
			rSum += r
			gSum += g
			bSum += b
		}
	}

	pixelCount := uint32((xMax - xMin) * (yMax - yMin))
	rAvr := rSum / pixelCount
	gAvr := gSum / pixelCount
	bAvr := bSum / pixelCount

	tile.averagePixel = color.RGBA{uint8(rAvr), uint8(gAvr), uint8(bAvr), 255}
	tile.cornerPixel = tileImage.At(tileImage.Bounds().Min.X, tileImage.Bounds().Min.Y)

	mutex.Lock()
	tileImages[filename] = tile.cornerPixel
	tiles[filename] = tile
	mutex.Unlock()

	//fmt.Printf("\t\t==> %v - %v\n", filename, pixelColour)

	//fmt.Printf(">>> %v (average) vs %v (corner) for filename %v\n\n", pixelColour, pixelColour1, filename)

	return
}

func main() {

	imageDir := "./images/"

	// get cli arguments
	//		get the image path from the cli ... -i
	//		get number of tiles in a row ...... -t
	imageFile := flag.String("i", "origImage.jpg", "Image path")
	tilesCount := flag.Int("t", 10, "Number of tiles along the image edge")

	flag.Parse()

	fmt.Println(*imageFile, *tilesCount)

	// prepare the tiles
	dir, err := os.Open(imageDir)
	if err != nil {
		log.Fatal(err)
	}
	defer dir.Close()

	tileFiles, err := dir.Readdir(-1)
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	var mutex sync.Mutex

	var imagePattern = regexp.MustCompile(`^.*\.(jpg|JPG|jpeg|JPEG)$`)

	tiles := make(map[string]*TileImage)
	tileImages := make(map[string]color.Color)

	tStart := time.Now()

	for _, tile := range tileFiles {

		//fmt.Printf("\t\t==> i=%v - tile=%s\n", i, tile.Name())

		wg.Add(1)
		go func(tile os.FileInfo) {
			defer wg.Done()

			filename := tile.Name()

			thisTile := &TileImage{}

			thisTile.getTileColours(&mutex, imagePattern, imageDir, filename, tiles, tileImages)
		}(tile)
	}

	wg.Wait()

	/*
		fmt.Printf("number of files : %d: \n", len(tileFiles))
		fmt.Printf("number of tiles : %d: \n", len(tiles))
		fmt.Printf("number of tiles : %d: \n", len(tileImages))
		fmt.Println("-------------------------------")
		fmt.Printf("%+v: \n", tiles)
		fmt.Println("-------------------------------")
		fmt.Printf("%+v: \n", tileImages)
		fmt.Println("-------------------------------")
		os.Exit(0)
	*/

	// read the file
	file, err := os.Open(*imageFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// encode into an image
	origImage, err := jpeg.Decode(bufio.NewReader(file))
	fmt.Printf("orig image %v: \n", origImage.At(origImage.Bounds().Min.X, origImage.Bounds().Min.Y))

	xMin := origImage.Bounds().Min.X
	xMax := origImage.Bounds().Max.X
	yMin := origImage.Bounds().Min.Y
	yMax := origImage.Bounds().Max.Y

	xDelta := int((xMax - xMin) / (*tilesCount))
	yDelta := int((yMax - yMin) / (*tilesCount))

	fmt.Printf("--> xMin=%v, xMax=%v, xDelta=%v\n\n", xMin, xMax, xDelta)
	fmt.Printf("--> yMin=%v, yMax=%v, yDelta=%v\n\n", yMin, yMax, yDelta)

	tEnd := time.Now()

	fmt.Printf("\t==> Tile processing took %v to run.\n", tEnd.Sub(tStart))

	tStart = time.Now()

	// change into a mosaic
	//		create a new empty image of the same size as the original one
	//		on which the tiles will be placed
	newImage := image.NewRGBA(image.Rect(xMin, yMin, xMax, yMax))

	// loop along x and y axes of the original one:
	// find the tile that is nearestTile in colour
	for y := yMin; y <= yMax; y += yDelta {
		for x := xMin; x <= xMax; x += xDelta {

			// find the average pixel colour of top left corner of each subimage
			rOrig, gOrig, bOrig, _ := origImage.At(x, y).RGBA()

			var nearestTile string
			var tileVectorDiff float64
			smallestDiff := 999999.0

			for tile, colour := range tileImages {

				r, g, b, _ := colour.RGBA()

				tileVectorDiff = math.Sqrt(math.Pow(float64(uint8(rOrig)-uint8(r)), 2) + math.Pow(float64(uint8(gOrig)-uint8(g)), 2) + math.Pow(float64(uint8(bOrig)-uint8(b)), 2))

				if tileVectorDiff < smallestDiff {
					smallestDiff = tileVectorDiff
					nearestTile = tile
				}

			}

			//fmt.Printf("--> x=%v, y=%v, tileVectorDiff=%v, nearestTile=%v\n", x, y, tileVectorDiff, nearestTile)

			// read the file

			file, err := os.Open(imageDir + nearestTile)
			if err != nil {
				log.Fatal(err)
			}

			// resize the tile and draw it on the new image
			tileImage, err := jpeg.Decode(bufio.NewReader(file))
			resizedTile := resize.Resize(uint(xDelta), 0, tileImage, resize.Lanczos3)

			//subImage := image.NewAlpha16(newImage.Bounds()).SubImage(image.Rect(x, y, x+xDelta, y+yDelta))
			// draw the tile into the new image
			draw.Draw(newImage, image.Rectangle{image.Point{x, y}, image.Point{x + xDelta, y + yDelta}}, resizedTile, image.Point{resizedTile.Bounds().Min.X, resizedTile.Bounds().Min.Y}, draw.Src)
			//fmt.Printf(">>> drawing at [%v, %v] : %v\n\n", x, y, nearestTile)

			file.Close()
		}
	}

	tEnd = time.Now()
	fmt.Printf("\t==> Mosaic processing took %v to run.\n", tEnd.Sub(tStart))

	// save the new finished image
	mosaicFile, err := os.Create("mosaic.jpg")
	if err != nil {
		log.Fatal(err)
	}
	defer mosaicFile.Close()

	// write new image to file
	var opt jpeg.Options
	opt.Quality = 80
	jpeg.Encode(mosaicFile, newImage, &opt)

}
