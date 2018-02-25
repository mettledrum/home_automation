package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gocv.io/x/gocv"
)

const (
	motion               = "motion"
	still                = "still"
	motionThresh float64 = 4000
	picTimeFmt           = "20060102150405"
)

func recordMotion(close <-chan (struct{})) {
	webcam, err := gocv.VideoCaptureDevice(0)
	if err != nil {
		panic("cannot open webcam")
	}
	defer webcam.Close()

	img := gocv.NewMat()
	defer img.Close()

	imgDelta := gocv.NewMat()
	defer imgDelta.Close()

	imgThresh := gocv.NewMat()
	defer imgThresh.Close()

	mog2 := gocv.NewBackgroundSubtractorMOG2()
	defer mog2.Close()

	if ok := webcam.Read(img); !ok {
		panic("cannot read from webcam")
	}

	n := time.Now().Format(picTimeFmt)
	saveFile := fmt.Sprintf("motion_%s.avi", n)

	writer, err := gocv.VideoWriterFile(saveFile, "MJPG", 25, img.Cols(), img.Rows())
	if err != nil {
		panic("cannot open video")
	}
	defer writer.Close()

	for {
		select {
		case <-close:
			return
		default:
			if ok := webcam.Read(img); !ok {
				panic("cannot read from webcam")
			}
			if img.Empty() {
				continue
			}

			mog2.Apply(img, imgDelta)
			gocv.Threshold(imgDelta, imgThresh, 25, 255, gocv.ThresholdBinary)

			kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))
			defer kernel.Close()

			gocv.Dilate(imgThresh, imgThresh, kernel)

			status := still
			contours := gocv.FindContours(imgThresh, gocv.RetrievalExternal, gocv.ChainApproxSimple)
			for _, c := range contours {
				area := gocv.ContourArea(c)
				if area < motionThresh {
					status = still
				} else {
					status = motion
				}
			}

			// record if motion is detected
			if status == motion {
				n := time.Now().Format(picTimeFmt)
				gocv.PutText(img, n, image.Pt(10, 20), gocv.FontHersheyPlain, 1.2, color.RGBA{0, 0, 0, 0}, 2)
				writer.Write(img)
			}
		}
	}
}

// call this on a cron, then send SIGINT or SIGTERM to write to file
func main() {
	sigs := make(chan os.Signal, 1)
	done := make(chan struct{}, 1)
	close := make(chan struct{}, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		recordMotion(close)
		done <- struct{}{}
	}()

	go func() {
		<-sigs
		close <- struct{}{}
	}()

	<-done
}
