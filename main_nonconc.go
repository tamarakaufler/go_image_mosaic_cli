// The application changes the provided image into a mosaic of images
// Usage:
//			./go_image_mosaic -i image_path -t 10
package main

import (
	"bufio"
	"errors"
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
	"time"

	"github.com/nfnt/resize"
)

func main() {

	imageDir := "./images/"

	// get cli arguments
	//		get the image path from the cli ... -i
	//		get number of tiles in a row ...... -t
	imageFile := flag.String("i", "origImage.jpg", "Image path")
	tilesCount := flag.Int("t", 10, "Number of tiles along the image edge")

	flag.Parse()

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

	tStart := time.Now()

	var imagePattern = regexp.MustCompile(`^.*\.(jpg|JPG|jpeg|JPEG)$`)

	tileImages := make(map[string]color.Color)

	for _, tile := range tileFiles {
		filename := tile.Name()

		pixelColour, err := getTileColours(imagePattern, imageDir, filename)

		if err == nil {
			tileImages[filename] = pixelColour
		}
	}

	tEnd := time.Now()

	fmt.Printf("\t==> Tile processing took %v to run.\n", tEnd.Sub(tStart))

	tStart = time.Now()

	// read the file
	file, err := os.Open(*imageFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// encode into an image
	origImage, err := jpeg.Decode(bufio.NewReader(file))
	//fmt.Printf("orig image %v: \n", origImage.At(origImage.Bounds().Min.X, origImage.Bounds().Min.Y))

	xMin := origImage.Bounds().Min.X
	xMax := origImage.Bounds().Max.X
	yMin := origImage.Bounds().Min.Y
	yMax := origImage.Bounds().Max.Y

	xDelta := int((xMax - xMin) / (*tilesCount))
	yDelta := int((yMax - yMin) / (*tilesCount))

	// change into a mosaic
	//		create a new empty image of the same size as the original one
	//		on which the tiles will be placed
	newImage := image.NewRGBA(image.Rect(xMin, yMin, xMax, yMax))

	tStart = time.Now()
	//		loop along x and y axes of the original one:

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

			// read the file
			file, err := os.Open(imageDir + nearestTile)
			if err != nil {
				log.Fatal(err)
			}

			// resize the tile and draw it on the new image
			tileImage, err := jpeg.Decode(bufio.NewReader(file))
			resizedTile := resize.Resize(uint(xDelta), 0, tileImage, resize.Lanczos3)

			// create a subImage (?)
			//subImage := image.NewAlpha16(newImage.Bounds()).SubImage(image.Rect(x, y, x+xDelta, y+yDelta))
			// draw the tile into the new image
			draw.Draw(newImage, image.Rectangle{image.Point{x, y}, image.Point{x + xDelta, y + yDelta}}, resizedTile, image.Point{resizedTile.Bounds().Min.X, resizedTile.Bounds().Min.Y}, draw.Src)

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

//
func getTileColours(imagePattern *regexp.Regexp, imageDir string, filename string) (color.Color, error) {

	if !imagePattern.MatchString(filename) {
		return nil, errors.New("Cannot get colour")
	}

	file, err := os.Open(imageDir + filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// encode into an image
	tileImage, err := jpeg.Decode(bufio.NewReader(file))
	if err != nil {
		return nil, err
	}

	pixelColour := tileImage.At(tileImage.Bounds().Min.X, tileImage.Bounds().Min.Y)

	return pixelColour, nil
}
