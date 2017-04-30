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
	"io/ioutil"
	"log"
	"math"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/nfnt/resize"
)

type Tile interface {
	getTileColour()
}

type TileImage struct {
	xMin        int
	yMin        int
	scaled      image.Image
	averageRGB  []float64
	cornerPixel color.Color
}

// calculates tile photo colour
func (tile *TileImage) getTileColour(mutex *sync.Mutex, imagePattern *regexp.Regexp,
	xDelta int, imageDir string, filename string,
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

	tile.xMin = tileImage.Bounds().Min.X
	tile.yMin = tileImage.Bounds().Min.Y

	resizedTile := resize.Resize(uint(xDelta), 0, tileImage, resize.Lanczos3)

	xMin := resizedTile.Bounds().Min.X
	xMax := resizedTile.Bounds().Max.X
	yMin := resizedTile.Bounds().Min.Y
	yMax := resizedTile.Bounds().Max.Y

	var rSum, gSum, bSum float64 = 0, 0, 0

	for y := yMin; y <= yMax; y++ {
		for x := xMin; x <= xMax; x++ {

			r, g, b, _ := tileImage.At(x, y).RGBA()
			rSum += float64(r)
			gSum += float64(g)
			bSum += float64(b)
		}
	}

	pixelCount := uint32((xMax - xMin) * (yMax - yMin))
	rAvr := rSum / float64(pixelCount)
	gAvr := gSum / float64(pixelCount)
	bAvr := bSum / float64(pixelCount)

	averageRGB := []float64{rAvr, gAvr, bAvr}

	tile.averageRGB = averageRGB
	tile.scaled = resizedTile

	mutex.Lock()
	tileImages[filename] = tile.cornerPixel
	tiles[filename] = tile
	mutex.Unlock()

	return
}

func getImageColour(image image.Image, xMin, yMin, xMax, yMax int) []float64 {

	var rSum, gSum, bSum float64 = 0.0, 0.0, 0.0

	for y := yMin; y <= yMax; y++ {
		for x := xMin; x <= xMax; x++ {

			r, g, b, _ := image.At(x, y).RGBA()
			rSum += float64(r)
			gSum += float64(g)
			bSum += float64(b)
		}
	}

	pixelCount := uint32((xMax - xMin) * (yMax - yMin))
	rAvr := rSum / float64(pixelCount)
	gAvr := gSum / float64(pixelCount)
	bAvr := bSum / float64(pixelCount)

	averageRGB := []float64{rAvr, gAvr, bAvr}

	return averageRGB
}

func main() {

	imageDir := "./images/"

	// get cli arguments
	imageFile := flag.String("i", "origImage.jpg", "Image path")
	tilesCount := flag.Int("t", 10, "Number of tiles along the image edge")

	flag.Parse()

	fmt.Println(*imageFile, *tilesCount)

	// prepare the tiles
	tileFiles, err := ioutil.ReadDir("./images")
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	var mutex sync.Mutex

	var imagePattern = regexp.MustCompile(`^.*\.(jpg|JPG|jpeg|JPEG)$`)

	tiles := make(map[string]*TileImage)
	tileImages := make(map[string]color.Color)

	// read the file
	file, err := os.Open(*imageFile)
	if err != nil {
		log.Fatal("#### ", err)
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

	if xDelta <= 0 || yDelta <= 0 {
		log.Fatalf("\nERROR: xDelta=%d, yDelta=%d\n\tmust be > 0\n\n", xDelta, yDelta)
	}

	fmt.Printf("--> xMin=%v, xMax=%v, xDelta=%v\n\n", xMin, xMax, xDelta)
	fmt.Printf("--> yMin=%v, yMax=%v, yDelta=%v\n\n", yMin, yMax, yDelta)

	tStart := time.Now()

	for _, tile := range tileFiles {

		wg.Add(1)
		go func(tile os.FileInfo) {
			defer wg.Done()

			filename := tile.Name()

			thisTile := &TileImage{}

			thisTile.getTileColour(&mutex, imagePattern, xDelta, imageDir, filename, tiles, tileImages)
		}(tile)
	}

	wg.Wait()

	tEnd := time.Now()
	fmt.Printf("Tile processing took %v to run.\n", tEnd.Sub(tStart))

	//fmt.Printf("number of files : %d: \n", len(tileFiles))
	//fmt.Printf("number of tiles : %d: \n", len(tiles))

	tStart = time.Now()

	// change into a mosaic
	//		create a new empty image of the same size as the original one
	//		on which the tiles will be placed
	newImage := image.NewRGBA(image.Rect(xMin, yMin, xMax, yMax))

	// loop along x and y axes of the original one:
	// find the tile that is nearestFilename in colour
	for y := yMin; y <= yMax; y += yDelta {
		for x := xMin; x <= xMax; x += xDelta {

			wg.Add(1)

			go func(x int, y int) {
				origRGB := getImageColour(origImage, x, y, x+xDelta, y+yDelta)

				var nearestFilename string
				var tileVectorDiff float64
				smallestDiff := 99999999.0

				for file, tile := range tiles {

					r, g, b := tile.averageRGB[0], tile.averageRGB[1], tile.averageRGB[2]

					tileVectorDiff = math.Sqrt(math.Pow((origRGB[0]-r), 2) + math.Pow((origRGB[1]-g), 2) + math.Pow((origRGB[2]-b), 2))

					if tileVectorDiff < smallestDiff {
						smallestDiff = tileVectorDiff
						nearestFilename = file
					}

				}

				// read the file
				scaledTile := tiles[nearestFilename].scaled
				xMin, yMin := tiles[nearestFilename].xMin, tiles[nearestFilename].yMin

				// draw the tile into the new image
				mutex.Lock()
				draw.Draw(newImage, image.Rectangle{image.Point{x, y}, image.Point{x + xDelta, y + yDelta}}, scaledTile, image.Point{xMin, yMin}, draw.Src)
				mutex.Unlock()

				wg.Done()
			}(x, y)
		}
	}

	wg.Wait()

	tEnd = time.Now()
	fmt.Printf("Mosaic processing took %v to run.\n", tEnd.Sub(tStart))

	// save the new finished image
	mosaicFile, err := os.Create("mosaic.jpg")
	if err != nil {
		log.Fatal(err)
	}
	defer mosaicFile.Close()

	var opt jpeg.Options
	opt.Quality = 80
	jpeg.Encode(mosaicFile, newImage, &opt)

	fmt.Println("END ...")

}
