package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/rwcarlsen/goexif/exif"
)

// LOADING BAR SECTION
type Bar struct {
	percent int64  // progress percentage done
	current int64  // current progress
	total   int64  // total value
	prog    string // actual progress bar to be printed
	fill    string // fill value
}

func (bar *Bar) New(start, total int64) {
	bar.current = start
	bar.total = total
	if bar.fill == "" {
		bar.fill = "="
	}
	bar.percent = bar.getPercent()
	for i := 0; i < int(bar.percent); i += 2 {
		bar.prog += bar.fill
	}
}

func (bar *Bar) getPercent() int64 {
	return int64(float32(bar.current) / float32(bar.total) * 100)
}

func (bar *Bar) Print(current int64) {
	bar.current = current
	last := bar.percent
	bar.percent = bar.getPercent()
	if bar.percent != last && bar.percent%2 == 0 {
		bar.prog += bar.fill
	}
	fmt.Printf(
		"\r[%-50s]%3d%% %8d/%d", bar.prog, bar.percent, bar.current, bar.total,
	)
}

func (bar *Bar) Finish() {
	fmt.Println()
}

// ORGANIZING SECTION
type Picture struct {
	path string
	date [3]int
}

// Quick parser for command-line arguments to pull the inputted directory that
// needs to be reorganized.
func parseArgs() string {
	args := os.Args[1:]
	if len(args) != 1 {
		fmt.Println("ERROR: Needed only the input of the directory to reorganize.")
		fmt.Println("Example: 'go run reorganize.go {PATH}'")
		os.Exit(0)
	}
	return args[0]
}

// Take in an array of files and output an array of file paths to just JPGs
func parseFiles(parent_path string, files []fs.FileInfo) [2][]string {
	var output [2][]string
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		file := f.Name()
		file_path := fmt.Sprintf("%s/%s", parent_path, file)
		if strings.Contains(file, ".JPG") || strings.Contains(file, ".jpg") {
			output[0] = append(output[0], file_path)
		} else {
			output[1] = append(output[1], file_path)
		}
	}
	return output
}

// Open file, decode exif info, pull date and return in the form of
// [year, month, day].
func getDate(picture string) [3]int {
	p, err := os.Open(picture)
	if err != nil {
		log.Fatal(err)
	}

	x, err := exif.Decode(p)
	if err != nil {
		// If there is an error in decoding then put it in error_files
		return [3]int{-1, -1, -1}
	}

	dt, err := x.DateTime()
	if err != nil {
		return [3]int{0, 0, 0}
	}
	return [3]int{dt.Year(), int(dt.Month()), dt.Day()}
}

// Loop through all pictures in our array, get their dates, and return an array
// of Picture objects that have the picture's path and date.
func setPictureDates(pictures []string) []Picture {
	var output []Picture
	for _, p := range pictures {
		var pic Picture
		pic.path = p
		pic.date = getDate(p)
		output = append(output, pic)
	}
	return output
}

// Take an inputted desired path, find where it stops existing, and then build
// the path from that point on.
func buildPath(desired_path string) {
	if strings.Contains(desired_path, "unknown_dates") {
		if _, err := os.Stat(desired_path); os.IsNotExist(err) {
			if e := os.Mkdir(desired_path, os.ModePerm); e != nil {
				log.Fatal(e)
			}
		}
	} else if strings.Contains(desired_path, "error_files") {
		if _, err := os.Stat(desired_path); os.IsNotExist(err) {
			if e := os.Mkdir(desired_path, os.ModePerm); e != nil {
				log.Fatal(e)
			}
		}
	} else {
		path := strings.Split(desired_path, "/")
		// Check if the year directory exists
		if _, err := os.Stat(strings.Join(path[:2], "/")); os.IsNotExist(err) {
			if e := os.Mkdir(strings.Join(path[:2], "/"), os.ModePerm); e != nil {
				log.Fatal(e)
			}
		}
		// Check if the month directory exists
		if _, err := os.Stat(strings.Join(path[:3], "/")); os.IsNotExist(err) {
			if e := os.Mkdir(strings.Join(path[:3], "/"), os.ModePerm); e != nil {
				log.Fatal(e)
			}
		}
		// Check if the day directory exists
		if _, err := os.Stat(strings.Join(path, "/")); os.IsNotExist(err) {
			if e := os.Mkdir(strings.Join(path, "/"), os.ModePerm); e != nil {
				log.Fatal(e)
			}
		}
	}
}

// Move the picture to it's appropriate year, month, and day directories.
func move(p Picture) {
	split_path := strings.Split(p.path, "/") // [base_path, file_name]
	var new_base_path string
	y, m, d := p.date[0], p.date[1], p.date[2]
	if y == 0 {
		// If the year indicates that the date is unknown, set the base path to unknown
		new_base_path = fmt.Sprintf("%s/unknown_dates", split_path[0])
	} else if y == -1 {
		// If there is an error decoding the files put in error
		new_base_path = fmt.Sprintf("%s/error_files", split_path[0])
	} else {
		new_base_path = fmt.Sprintf("%s/%d/%d/%d", split_path[0], y, m, d)
	}

	// Make sure that the directory we want to move the picture to exists.
	if _, err := os.Stat(new_base_path); os.IsNotExist(err) {
		// If it doesn't exist, build it.
		buildPath(new_base_path)
	}
	// Move the file to the new directory
	new_filename := fmt.Sprintf("%s/%s", new_base_path, split_path[1])
	os.Rename(p.path, new_filename)
}

// Trigger function that controls the progress bar, getting picture dates, and
// moving all files around.
func organize(files [2][]string) {
	// Start the total file count
	count := int64(0)
	total := int64(len(files[0]) + len(files[1]))
	fmt.Printf("Starting to organize. %d files to organize.\n", total)
	// Start up the progress bar
	var bar Bar
	bar.New(count, total)
	// Organize all of the photos
	pics := setPictureDates(files[0])
	for _, p := range pics {
		move(p)
		// Increment progress bar
		bar.Print(count)
		count++
	}

	// Organize all other files
	base_path := strings.Split(files[1][0], "/")[0]
	os.Mkdir(base_path+"/others", os.ModePerm)
	for _, f := range files[1] {
		// CREATE OLD AND NEW FILENAMES AND RUN TESTS TO ENSURE PROPER PATHS ARE SET
		iso_name := strings.Split(f, "/")[1]
		new_filename := fmt.Sprintf("%s/others/%s", base_path, iso_name)
		os.Rename(f, new_filename)
		// Increment progress bar
		bar.Print(count)
		count++
	}
	// End progress bar
	bar.Finish()
}

func main() {
	// Get path of directory to reorganize from command line arguments
	path := parseArgs()
	// Get all of the files in the directory
	files, err := ioutil.ReadDir(path)
	if err != nil {
		fmt.Println("ERROR: MAIN")
		log.Fatal(err)
	}
	// Seperate the picture files from all other files
	all_files := parseFiles(path, files)
	// Pass all files to the organizing function that takes care of the rest.
	organize(all_files)
}
