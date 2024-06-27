package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kardianos/osext"
	"github.com/schollz/progressbar/v3"
)

type SpecificFiles struct {
	FilePath         string   `json:"filePath"`
	GenerationScript []string `json:"generationScript"`
}

type Rules struct {
	Directories   []string                   `json:"directories"`
	ExcludedFiles []string                   `json:"excludedFiles"`
	Destination   string                     `json:"destination"`
	SpecificFiles map[string][]SpecificFiles `json:"specificFiles"`
}

func main() {
	// get the flag to generate the zip files with or without the containing folder
	containingFolder := flag.Bool("folder", false, "Generate the zip files with the containing folder")
	flag.Parse()

	// show options base on flags
	fmt.Println("Generate zip files with containing folder: ", *containingFolder)
	folderPath, pathErr := osext.ExecutableFolder()
	if pathErr != nil {
		panic(pathErr)
	}
	fmt.Println("Running on this folder: ", folderPath)
	buildData := getRules(folderPath)
	excludedFiles := make(map[string]bool)
	now := time.Now()
	cTime := now.Format("2006-01-02 15-04-05")
	for _, num := range buildData.ExcludedFiles {
		excludedFiles[num] = true
	}
	fmt.Println("running directories")
	for i := 0; i < len(buildData.Directories); i++ {
		currentFolder := ""
		zipName := ""
		if dirExists(folderPath + "/" + buildData.Directories[i]) {
			currentFolder = folderPath + "/"
			zipName = folderPath + "/" + buildData.Directories[i]
		} else {
			currentFolder = "./"
			zipName = buildData.Directories[i]
		}
		fmt.Println("Validating specific directory ", buildData.SpecificFiles[buildData.Directories[i]])
		//Not needed for the MVP
		//docker compose can generate the metadata missing file
		//validateAndGenerateRequiredFiles(buildData.SpecificFiles[buildData.Directories[i]])
		fmt.Println("List files in ", buildData.Directories[i])
		filesToCompres := listDirectoryFiles(currentFolder, buildData.Directories[i], excludedFiles)
		//fmt.Println(filesToCompres)
		//For the MVP loop again the listed files and compress them
		//For the Version 2, compress them in parallel as they are listed
		fmt.Println("creating zip archive...")
		fmt.Println(buildData.Directories[i] + "-" + cTime + ".zip")
		archive, err := os.Create(zipName + "-" + cTime + ".zip")
		if err != nil {
			panic(err)
		}
		defer archive.Close()
		zipWriter := zip.NewWriter(archive)
		//initialize bar
		bar := progressbar.Default(int64(len(filesToCompres)))
		//Write the files on the corresponding zip file
		for j := 0; j < len(filesToCompres); j++ {
			// fmt.Println(filesToCompres[j])
			// fmt.Println("Opening file: " + filesToCompres[j])
			f, err := os.Open(currentFolder + filesToCompres[j])
			if err != nil {
				fmt.Println(err)
			}
			defer f.Close()
			// removing the directory name from the file name
			fileName := filesToCompres[j]
			if !*containingFolder {
				fileName = strings.Replace(filesToCompres[j], buildData.Directories[i]+"/", "", 1)
			}

			w, err := zipWriter.Create(fileName)
			if err != nil {
				fmt.Println(err)
			}
			if _, err := io.Copy(w, f); err != nil {
				fmt.Println(err)
				panic(err)
			}
			bar.Add(1)
		}
		fmt.Println("Close Zip Archive...")
		zipWriter.Close()

	}
}

func getRules(filePath string) Rules {
	fmt.Println("Prepare to open rules...")
	jsonFile, err := os.Open(filePath + "/rules.json")
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
		newjsonFile, newerr := os.Open("./rules.json")
		if newerr != nil {
			panic(newerr)
		}
		jsonFile = newjsonFile
	}
	fmt.Println("Successfully Opened rules.json")
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	fileInBytes, _ := io.ReadAll(jsonFile)
	var buildData Rules
	json.Unmarshal(fileInBytes, &buildData)
	return buildData
}

func listDirectoryFiles(currentPath string, directory string, excludedFiles map[string]bool) []string {
	files, err := os.ReadDir(currentPath + directory)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	var res []string

	for _, file := range files {
		// validate with a regex the file name
		// if the file is not valid, skip it
		// take the file.Name() and validate using the exlcudedFiles map
		if _, ok := excludedFiles[file.Name()]; ok {
			fmt.Println("Excluded file: " + file.Name())
		} else {
			if file.IsDir() {
				innerDir := listDirectoryFiles(currentPath, directory+"/"+file.Name(), excludedFiles)
				res = append(res, innerDir...)
			} else {
				res = append(res, directory+"/"+file.Name())
			}
		}

	}
	return res
}

func dirExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	panic(err)
}

func validateAndGenerateRequiredFiles(files []SpecificFiles) bool {
	res := true
	for _, num := range files {
		fmt.Println("reading ", num.FilePath)
		_, err := os.ReadDir(num.FilePath)
		if err != nil {
			fmt.Println("file not found")
			fmt.Print("executing command:", strings.Join(num.GenerationScript[:], ","))
			cmd := exec.Command(num.GenerationScript[0], num.GenerationScript[1:]...)
			stdout, execErr := cmd.Output()
			if execErr != nil {
				fmt.Println(err.Error())
				res = false
			}
			fmt.Println(string(stdout))
		}

	}
	return res
}
