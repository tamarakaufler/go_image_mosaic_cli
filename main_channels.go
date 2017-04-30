// The application changes the provided image into a mosaic of images
//		* concurrent implementation with goroutines
//		* uses average tile pixel value
// usage:
//			./go_image_mosaic -i image_path -t 10
package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/nfnt/resize"
	"image"
	"image/draw"
	"image/jpeg"
	"io/ioutil"
	"log"
	"math"
	"os"
	"regexp"
	"sync"
	"time"
)

type Tile interface {
	getTileColour()
}

type TileImage struct {
	filename   string
	xMin       int
	yMin       int
	scaled     image.Image
	averageRGB []float64
	//cornerPixel color.Color
}

type tileMessage struct {
	filename string
	tile     *TileImage
}

// calculates tile photo colour
func getTileColour(wg *sync.WaitGroup, xDelta int, filename string, tileImage image.Image,
	tileData chan tileMessage) {

	resizedTile := resize.Resize(uint(xDelta), 0, tileImage, resize.Lanczos3)

	xMin := resizedTile.Bounds().Min.X
	xMax := resizedTile.Bounds().Max.X
	yMin := resizedTile.Bounds().Min.Y
	yMax := resizedTile.Bounds().Max.Y

	averageRGB := getImageColour(resizedTile, xMin, yMin, xMax, yMax)

	tile := &TileImage{
		xMin:       tileImage.Bounds().Min.X,
		yMin:       tileImage.Bounds().Min.Y,
		averageRGB: averageRGB,
		scaled:     resizedTile,
	}

	message := tileMessage{
		filename: filename,
		tile:     tile,
	}

	tileData <- message

	//fmt.Printf("\t\tgoroutines = %d - processing filename %s: %v\n", runtime.NumGoroutine(), filename, message)

	wg.Done()
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
	//		get the image path from the cli ... -i
	//		get number of tiles in a row ...... -t
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

	//tiles := make(map[string]*TileImage)

	// read the file
	file, err := os.Open(*imageFile)
	if err != nil {
		log.Fatal("#### ", err)
	}

	// encode into an image
	origImage, err := jpeg.Decode(bufio.NewReader(file))
	//fmt.Printf("orig image %v: \n", origImage.At(origImage.Bounds().Min.X, origImage.Bounds().Min.Y))

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

	tileData := make(chan tileMessage, len(tileFiles))

	tStart := time.Now()

	// loop through files
	// goroutine to process each file
	//		create TileImage struct to hold tile data
	//		find average pixel values
	//		send to channel
	for _, fileTile := range tileFiles {
		filename := fileTile.Name()

		//fmt.Printf("\t\t==> i=%d - tile=%s\n", i, fileTile.Name())

		if !imagePattern.MatchString(filename) {
			continue
		}

		file, err := os.Open(imageDir + filename)
		if err != nil {
			continue
		}

		// encode into an image
		tileImage, err := jpeg.Decode(bufio.NewReader(file))
		if err != nil {
			continue
		}

		wg.Add(1)

		go func(wg *sync.WaitGroup, xDelta int, filename string, tileImage image.Image,
			tileData chan tileMessage) {
			getTileColour(wg, xDelta, filename, tileImage, tileData)

		}(&wg, xDelta, filename, tileImage, tileData)

	}

	wg.Wait()

	// close the channel after processing all the tiles
	close(tileData)

	// goroutine to create the mosaic
	//		read data from tileFileData channel
	//			lock to draw the tile
	//
	//
	tiles := make(map[string]*TileImage)

	for m := range tileData {
		tiles[m.filename] = m.tile
	}

	//fmt.Printf("Tiles: %+v\n", tiles)
	//fmt.Printf("Number of files = %d\n", len(tileFiles))
	//fmt.Printf("Number of tiles keys = %d\n", len(tiles))

	tEnd := time.Now()

	fmt.Printf("\t==> Tile processing took %v to run.\n", tEnd.Sub(tStart))

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
				// find the average pixel colour of top left corner of each subimage
				//rOrig, gOrig, bOrig, _ := origImage.At(x, y).RGBA()
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

				// draw the tile into the new image
				mutex.Lock()
				draw.Draw(newImage, image.Rectangle{image.Point{x, y}, image.Point{x + xDelta, y + yDelta}}, tiles[nearestFilename].scaled, image.Point{xMin, yMin}, draw.Src)
				mutex.Unlock()

				wg.Done()
			}(x, y)
		}
	}

	wg.Wait()

	tEnd = time.Now()
	fmt.Printf("\t==> Mosaic processing took %v to run.\n", tEnd.Sub(tStart))

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
