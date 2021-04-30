package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type entry struct {
	offset int
	length int
}

type PatchInfo struct {
	Name    string
	Patches []PatchData
}

type PatchData struct {
	Find      string
	FindRegex *regexp.Regexp
	Replace   string
}

const UINT32_LENGTH = 4

var mode string
var bundlePath string
var outputFilename string
var outputDir string
var patchesDir string

func init() {
	flag.StringVar(&mode, "m", "unpack", "Set mode (pack/unpack/patch)")
	flag.StringVar(&bundlePath, "p", "", "Set the jsbundle path")
	flag.StringVar(&outputFilename, "n", "patched.jsbundle", "Set the output filename")
	flag.StringVar(&outputDir, "o", "out", "Set the output dir")
	flag.StringVar(&patchesDir, "d", "", "Set the folder for patches")

	flag.Parse()

	if mode == "unpack" || mode == "patch" {
		if bundlePath == "" {
			fmt.Println("Please set the bundle path.")
			os.Exit(0)
		}
	}

	if mode == "patch" {
		if patchesDir == "" {
			fmt.Println("Please set the patches folder.")
			os.Exit(0)
		}
	}
}

func main() {
	fmt.Println("Starting jsbundletools...")

	if mode == "unpack" {
		modules := readModulesFromBundle()
		unpack(modules)

		return
	}

	if mode == "pack" {
		modules := readModulesFromFolder()
		pack(modules)

		return
	}

	if mode == "patch" {
		modules := readModulesFromBundle()
		patch(modules)
		pack(modules)

		return
	}

	fmt.Println("Mode not available.")
}

// Write bytes to file at offset
func writeToFile(file *os.File, data uint32, offset int) {
	buffer := make([]byte, UINT32_LENGTH)
	binary.LittleEndian.PutUint32(buffer, data)
	file.WriteAt(buffer, int64(offset))
}

// Read bytes from a file
func readFile(file *os.File, offset int) uint32 {
	bytes := make([]byte, UINT32_LENGTH)

	file.Seek(int64(offset), 0)

	data, err := file.Read(bytes)
	if err != nil {
		panic(err)
	}

	return binary.LittleEndian.Uint32(bytes[:data])
}

// Read bytes from offset
func readFileAtOffset(file *os.File, offset int, size int) []byte {
	bytes := make([]byte, size)
	file.Seek(int64(offset), 0)

	data, err := file.Read(bytes)
	if err != nil {
		panic(err)
	}

	return bytes[:data]
}

// Check if the file has the magic number
func checkMagicNumber(magicNumber uint32) {
	if magicNumber != 0xfb0bd1e5 {
		fmt.Println("Magic number not found.")
		os.Exit(0)
	}
}

// Read the modules from the bundle and return a modules map
func readModulesFromBundle() *map[string][]byte {
	bundleFile, err := os.Open(bundlePath)
	if err != nil {
		panic(err)
	}

	defer bundleFile.Close()

	modules := map[string][]byte{}

	magicNumber := readFile(bundleFile, 0)
	checkMagicNumber(magicNumber)

	entryCount := readFile(bundleFile, UINT32_LENGTH)
	startupCountLength := int(readFile(bundleFile, UINT32_LENGTH*2))

	entries := map[int]entry{}

	entryTableStart := UINT32_LENGTH * 3
	position := entryTableStart

	for entryId := 0; entryId < int(entryCount); entryId++ {
		entry := entry{
			offset: int(readFile(bundleFile, position)),
			length: int(readFile(bundleFile, position+UINT32_LENGTH)),
		}

		entries[entryId] = entry
		position += UINT32_LENGTH * 2
	}

	moduleStart := position

	for index, entry := range entries {
		start := moduleStart + entry.offset

		moduleData := readFileAtOffset(bundleFile, start, entry.length)
		if len(moduleData) > 0 {
			moduleData = moduleData[:len(moduleData)-1]
		}

		modules[strconv.Itoa(index)] = moduleData
	}

	startupSize := (moduleStart + startupCountLength - 1) - moduleStart
	modules["startup"] = readFileAtOffset(bundleFile, moduleStart, startupSize)

	return &modules
}

// Read the modules from a folder
func readModulesFromFolder() *map[string][]byte {
	files, err := os.ReadDir(outputDir)
	if err != nil {
		panic(err)
	}

	modules := map[string][]byte{}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".js") {
			continue
		}

		id := strings.TrimSuffix(file.Name(), ".js")
		data, err := os.ReadFile(fmt.Sprintf("%v/%v", outputDir, file.Name()))
		if err != nil {
			panic(err)
		}

		modules[id] = data
	}

	return &modules
}

// Unpack a list of modules to output folder
func unpack(modules *map[string][]byte) {
	fmt.Println("Unpacking", bundlePath)

	os.Mkdir(outputDir, 0755)

	for index, content := range *modules {
		f, err := os.Create(fmt.Sprintf("%v/%v.js", outputDir, index))
		if err != nil {
			panic(err)
		}

		f.WriteString(string(content))
		f.Close()
	}

	fmt.Println("Done!")
}

// Apply patches a list of modules
func patch(modules *map[string][]byte) {
	patchesFolders, err := os.ReadDir(patchesDir)
	if err != nil {
		panic(err)
	}

	patches := []PatchInfo{}

	for _, patchFile := range patchesFolders {
		if !strings.HasSuffix(patchFile.Name(), ".json") {
			continue
		}

		patchFileContent, err := os.ReadFile(fmt.Sprintf("%v/%v", patchesDir, patchFile.Name()))
		if err != nil {
			panic(err)
		}

		var info PatchInfo
		json.Unmarshal(patchFileContent, &info)
		info.Name = strings.Replace(patchFile.Name(), ".json", "", -1)

		for index, patch := range info.Patches {
			info.Patches[index].FindRegex = regexp.MustCompile(patch.Find)
		}

		patches = append(patches, info)
	}

	for _, info := range patches {
		fmt.Printf("Applying patches for %v\n", info.Name)
		for moduleID := range *modules {
			for _, patch := range info.Patches {
				module := (*modules)[moduleID]
				(*modules)[moduleID] = []byte(patch.FindRegex.ReplaceAllString(string(module), patch.Replace))
			}
		}
	}

	fmt.Println("Patches were applied!")
}

// Pack a list of modules into a jsbundle file
func pack(modules *map[string][]byte) {
	fmt.Println("Repacking jsbundle.")

	startup := (*modules)["startup"]
	delete(*modules, "startup")

	entries := map[string]entry{}
	offset := len(startup) + 1

	for moduleId, content := range *modules {
		entries[moduleId] = entry{
			offset: offset,
			length: len(content) + 1,
		}

		offset += entries[moduleId].length
	}

	entryCount := len(entries)
	length := offset + UINT32_LENGTH*3 + entryCount*2*UINT32_LENGTH

	outputFile, err := os.Create(outputFilename)
	if err != nil {
		panic(err)
	}

	os.Truncate(outputFilename, int64(length))

	defer outputFile.Close()

	writeToFile(outputFile, 0xfb0bd1e5, 0)
	writeToFile(outputFile, uint32(entryCount), UINT32_LENGTH)
	writeToFile(outputFile, uint32(len(startup)+1), UINT32_LENGTH*2)

	tableStart := UINT32_LENGTH * 3
	moduleStart := tableStart + entryCount*UINT32_LENGTH*2
	position := tableStart

	for i := 0; i < len(entries); i++ {
		entryId := strconv.Itoa(i)
		entry := entries[entryId]

		writeToFile(outputFile, uint32(entry.offset), position)
		writeToFile(outputFile, uint32(entry.length), position+UINT32_LENGTH)
		position += UINT32_LENGTH * 2

		outputFile.WriteAt((*modules)[entryId], int64(moduleStart+entry.offset))
	}

	outputFile.WriteAt(startup, int64(moduleStart))
	outputFile.WriteAt([]byte{0}, int64(moduleStart+len(startup)))

	fmt.Println("jsbundle has been created.")
}
