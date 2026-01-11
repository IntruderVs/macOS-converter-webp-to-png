package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/webp"
)

func convertWebPToPNG(inputPath, outputPath string) error {
	// Читаем файл полностью в память для более надежного декодирования
	// (некоторые WebP файлы могут не работать с потоковым чтением)
	inputData, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("не удалось прочитать файл: %w", err)
	}

	// Пытаемся декодировать изображение - сначала как WebP, затем как JPEG
	// (некоторые файлы с расширением .webp на самом деле являются JPEG)
	var img image.Image

	// Пробуем декодировать как WebP
	img, err = webp.Decode(bytes.NewReader(inputData))
	if err != nil {
		// Если не получилось, пробуем декодировать как JPEG
		img, err = jpeg.Decode(bytes.NewReader(inputData))
		if err != nil {
			return fmt.Errorf("не удалось декодировать изображение (пробовались форматы WebP и JPEG): %w", err)
		}
	}

	// Создаем выходной файл только после успешного декодирования
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("не удалось создать выходной файл: %w", err)
	}
	defer outputFile.Close()

	// Кодируем в PNG
	err = png.Encode(outputFile, img)
	if err != nil {
		outputFile.Close()
		os.Remove(outputPath) // Удаляем неполный файл при ошибке
		return fmt.Errorf("не удалось закодировать PNG: %w", err)
	}

	// Синхронизируем данные на диск
	err = outputFile.Sync()
	if err != nil {
		return fmt.Errorf("не удалось синхронизировать файл: %w", err)
	}

	return nil
}

func convertDirectory(inputDir, outputDir string) error {
	// Читаем директорию
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return fmt.Errorf("не удалось прочитать директорию: %w", err)
	}

	var webpFiles []string
	for _, entry := range entries {
		if !entry.IsDir() {
			name := strings.ToLower(entry.Name())
			if strings.HasSuffix(name, ".webp") {
				webpFiles = append(webpFiles, entry.Name())
			}
		}
	}

	if len(webpFiles) == 0 {
		fmt.Printf("WebP файлы не найдены в директории: %s\n", inputDir)
		return nil
	}

	fmt.Printf("Найдено WebP файлов: %d\n", len(webpFiles))

	// Создаем выходную директорию, если нужно
	if outputDir != "" && outputDir != inputDir {
		err = os.MkdirAll(outputDir, 0755)
		if err != nil {
			return fmt.Errorf("не удалось создать выходную директорию: %w", err)
		}
	}

	// Слайс для хранения ошибок конвертации
	type conversionError struct {
		fileName string
		errorMsg string
	}
	var errors []conversionError

	successCount := 0
	for _, webpFile := range webpFiles {
		inputPath := filepath.Join(inputDir, webpFile)
		pngName := strings.TrimSuffix(webpFile, filepath.Ext(webpFile)) + ".png"

		var outputPath string
		if outputDir != "" {
			outputPath = filepath.Join(outputDir, pngName)
		} else {
			outputPath = filepath.Join(inputDir, pngName)
		}

		err := convertWebPToPNG(inputPath, outputPath)
		if err != nil {
			errorMsg := fmt.Sprintf("Ошибка при конвертации %s: %v", webpFile, err)
			fmt.Fprintf(os.Stderr, "%s\n", errorMsg)
			errors = append(errors, conversionError{fileName: webpFile, errorMsg: errorMsg})
			continue
		}

		// Проверяем, что выходной файл действительно создан и не пустой
		outputInfo, err := os.Stat(outputPath)
		if err != nil {
			errorMsg := fmt.Sprintf("Выходной файл %s не найден после конвертации %s", outputPath, webpFile)
			fmt.Fprintf(os.Stderr, "Ошибка: %s\n", errorMsg)
			errors = append(errors, conversionError{fileName: webpFile, errorMsg: errorMsg})
			continue
		}
		if outputInfo.Size() == 0 {
			errorMsg := fmt.Sprintf("Выходной файл %s пустой после конвертации %s", outputPath, webpFile)
			fmt.Fprintf(os.Stderr, "Ошибка: %s\n", errorMsg)
			os.Remove(outputPath) // Удаляем пустой файл
			errors = append(errors, conversionError{fileName: webpFile, errorMsg: errorMsg})
			continue
		}

		fmt.Printf("✓ Конвертировано: %s -> %s\n", webpFile, pngName)
		successCount++
	}

	// Создаем файл отчета с ошибками, если есть ошибки
	if len(errors) > 0 {
		reportDir := outputDir
		if reportDir == "" {
			reportDir = inputDir
		}
		reportPath := filepath.Join(reportDir, "conversion_errors.txt")

		reportFile, err := os.Create(reportPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Предупреждение: не удалось создать файл отчета: %v\n", err)
		} else {
			defer reportFile.Close()

			fmt.Fprintf(reportFile, "Отчет об ошибках конвертации\n")
			fmt.Fprintf(reportFile, "================================\n\n")
			fmt.Fprintf(reportFile, "Всего ошибок: %d\n\n", len(errors))

			for i, convErr := range errors {
				fmt.Fprintf(reportFile, "%d. Файл: %s\n", i+1, convErr.fileName)
				fmt.Fprintf(reportFile, "   Ошибка: %s\n\n", convErr.errorMsg)
			}

			fmt.Printf("\nФайл отчета с ошибками создан: %s\n", reportPath)
		}
	}

	fmt.Printf("\nКонвертация завершена. Успешно: %d из %d\n", successCount, len(webpFiles))
	return nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "WebP to PNG Converter\n")
		fmt.Fprintf(os.Stderr, "=====================\n\n")
		fmt.Fprintf(os.Stderr, "Использование:\n")
		fmt.Fprintf(os.Stderr, "  %s <входной_файл.webp> [выходной_файл.png]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s <директория> [выходная_директория]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Примеры:\n")
		fmt.Fprintf(os.Stderr, "  %s image.webp\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s image.webp output.png\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s ./images\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s ./images ./converted\n", os.Args[0])
	}

	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	input := args[0]
	output := ""
	if len(args) > 1 {
		output = args[1]
	}

	// Проверяем, что входной путь существует
	info, err := os.Stat(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}

	if info.IsDir() {
		// Конвертируем все WebP файлы в директории
		err := convertDirectory(input, output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Конвертируем один файл
		if output == "" {
			// Если выходной файл не указан, создаем PNG с тем же именем
			output = strings.TrimSuffix(input, filepath.Ext(input)) + ".png"
		}

		err := convertWebPToPNG(input, output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Успешно конвертировано: %s -> %s\n", filepath.Base(input), filepath.Base(output))
	}
}
