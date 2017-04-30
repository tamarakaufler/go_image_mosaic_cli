# go_image_mosaic_cli
CLI implementation of creating an image mosaic

go run main.go -i origImage.jpg -t 8

# Performance statistics

## main_nonconc.go (first implementation)
	==> Tile processing took 32.921956ms to run.
	==> Mosaic processing took 201.820631ms to run.

## main_conc.go (first concurrent implementation)
	==> Tile processing took 78.95146ms to run.
	==> Mosaic processing took 202.431574ms to run.

## main_mutex.go (improved concurrent implementation)
	==> Tile processing took 69.299504ms to run.
	==> Mosaic processing took 2.561721ms to run.

## main_channels.go (improved concurrent implementation with channels )
	==> Tile processing took 68.388311ms to run.
	==> Mosaic processing took 2.634203ms to run.
    
