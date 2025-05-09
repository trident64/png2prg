package main

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"syscall/js"

	"github.com/staD020/png2prg" // Import the png2prg package
)

func main() {
	// Register both functions
	js.Global().Set("convertPngToPrg", js.FuncOf(fetchImageData))
	js.Global().Set("convertImageData", js.FuncOf(convertDirectImageData))

	fmt.Println("PNG2PRG WebAssembly initialized")

	// Keep the Go program running
	<-make(chan bool)
}

// fetchImageData - fetches an image and returns a Promise
func fetchImageData(this js.Value, args []js.Value) interface{} {
	// Check if URL is passed directly to the function
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return map[string]interface{}{
			"error": "No URL provided",
		}
	}

	// Get URL from the function arguments
	targetUrl := args[0].String()

	// Get options from the function arguments
	var jsOptions js.Value
	if len(args) > 1 && args[1].Type() == js.TypeObject {
		jsOptions = args[1]
	}

	fmt.Printf("Fetching image from: %s\n", targetUrl)

	// Create a JavaScript Promise
	handler := js.FuncOf(func(this js.Value, handlerArgs []js.Value) interface{} {
		resolve := handlerArgs[0]
		reject := handlerArgs[1]

		// Try different CORS proxies in sequence
		tryNextProxy([]string{
			"https://corsproxy.io/?url=" + url.QueryEscape(targetUrl),
			"https://corsproxy.org/?" + url.QueryEscape(targetUrl),
			"https://api.allorigins.win/raw?url=" + url.QueryEscape(targetUrl), // Move this to the end
			"https://cors-anywhere.herokuapp.com/" + targetUrl,
			targetUrl, // Try direct URL as last resort
		}, 0, resolve, reject, jsOptions)

		return nil
	})

	// Return a new Promise to JavaScript
	return js.Global().Get("Promise").New(handler)
}

// tryNextProxy attempts to fetch through each proxy in the list
func tryNextProxy(proxyUrls []string, index int, resolve, reject js.Value, jsOptions js.Value) {
	// Check if we've exhausted all proxies
	if index >= len(proxyUrls) {
		reject.Invoke("All CORS proxies failed")
		return
	}

	proxyUrl := proxyUrls[index]
	fmt.Printf("Trying proxy URL: %s\n", proxyUrl)

	// Fetch the image through the proxy
	fetchPromise := js.Global().Call("fetch", proxyUrl)

	// Handle the response
	processResponse := js.FuncOf(func(this js.Value, responseArgs []js.Value) interface{} {
		response := responseArgs[0]

		// Check if response is OK
		if !response.Get("ok").Bool() {
			fmt.Printf("Proxy %s failed with status: %d\n", proxyUrl, response.Get("status").Int())
			// Try the next proxy
			tryNextProxy(proxyUrls, index+1, resolve, reject, jsOptions)
			return nil
		}

		// Get array buffer
		response.Call("arrayBuffer").Call("then",
			js.FuncOf(func(this js.Value, bufferArgs []js.Value) interface{} {
				buffer := bufferArgs[0]

				// Create a Uint8Array from the ArrayBuffer
				uint8Array := js.Global().Get("Uint8Array").New(buffer)

				// Convert buffer to Go byte slice
				bufferBytes := make([]byte, uint8Array.Length())
				js.CopyBytesToGo(bufferBytes, uint8Array)

				fmt.Printf("Image data received: %d bytes\n", len(bufferBytes))

				// Save original stdout
				oldStdout := os.Stdout
				// Create a pipe to capture output
				r, w, _ := os.Pipe()
				os.Stdout = w

				// Create options for PNG2PRG
				options := png2prg.Options{
					Display: true,  // Include displayer
					Quiet:   false, // Enable verbose output
				}

				// Apply JavaScript options if provided
				if jsOptions.Truthy() {
					// Set graphics mode if specified
					if mode := jsOptions.Get("mode"); mode.Type() == js.TypeString && mode.String() != "" {
						options.GraphicsMode = mode.String()
						fmt.Printf("Setting graphics mode to: %s\n", options.GraphicsMode)
					}

					// Set bitpair colors if specified
					if bpc := jsOptions.Get("bitpairColors"); bpc.Type() == js.TypeString && bpc.String() != "" {
						options.BitpairColorsString = bpc.String()
						fmt.Printf("Setting bitpair colors to: %s\n", options.BitpairColorsString)
					}

					// Set bitpair colors 2 if specified
					if bpc2 := jsOptions.Get("bitpairColors2"); bpc2.Type() == js.TypeString && bpc2.String() != "" {
						options.BitpairColorsString2 = bpc2.String()
						fmt.Printf("Setting bitpair colors 2 to: %s\n", options.BitpairColorsString2)
					}

					// Set bitpair colors 3 if specified
					if bpc3 := jsOptions.Get("bitpairColors3"); bpc3.Type() == js.TypeString && bpc3.String() != "" {
						options.BitpairColorsString3 = bpc3.String()
						fmt.Printf("Setting bitpair colors 3 to: %s\n", options.BitpairColorsString3)
					}

					// Set display option
					if display := jsOptions.Get("display"); display.Type() == js.TypeBoolean {
						options.Display = display.Bool()
						fmt.Printf("Setting display option to: %v\n", options.Display)
					}

					// Set brute force option
					if bf := jsOptions.Get("bruteForce"); bf.Type() == js.TypeBoolean {
						options.BruteForce = bf.Bool()
						fmt.Printf("Setting brute force option to: %v\n", options.BruteForce)
					}

					// Set interlace option
					if interlace := jsOptions.Get("interlace"); interlace.Type() == js.TypeBoolean {
						options.Interlace = interlace.Bool()
						fmt.Printf("Setting interlace option to: %v\n", options.Interlace)
					}

					// Set force border color
					if forceBorderColor := jsOptions.Get("forceBorderColor"); forceBorderColor.Type() == js.TypeNumber {
						options.ForceBorderColor = forceBorderColor.Int()
						fmt.Printf("Setting force border color to: %d\n", options.ForceBorderColor)
					}

					// Set force X offset
					if forceXOffset := jsOptions.Get("forceXOffset"); forceXOffset.Type() == js.TypeNumber {
						options.ForceXOffset = forceXOffset.Int()
						fmt.Printf("Setting force X offset to: %d\n", options.ForceXOffset)
					}

					// Set force Y offset
					if forceYOffset := jsOptions.Get("forceYOffset"); forceYOffset.Type() == js.TypeNumber {
						options.ForceYOffset = forceYOffset.Int()
						fmt.Printf("Setting force Y offset to: %d\n", options.ForceYOffset)
					}

					// Set no pack chars option
					if noPackChars := jsOptions.Get("noPackChars"); noPackChars.Type() == js.TypeBoolean {
						options.NoPackChars = noPackChars.Bool()
						fmt.Printf("Setting no pack chars option to: %v\n", options.NoPackChars)
					}

					// Set no pack empty char option
					if noPackEmptyChar := jsOptions.Get("noPackEmptyChar"); noPackEmptyChar.Type() == js.TypeBoolean {
						options.NoPackEmptyChar = noPackEmptyChar.Bool()
						fmt.Printf("Setting no pack empty char option to: %v\n", options.NoPackEmptyChar)
					}

					// Set force pack empty char option
					if forcePackEmptyChar := jsOptions.Get("forcePackEmptyChar"); forcePackEmptyChar.Type() == js.TypeBoolean {
						options.ForcePackEmptyChar = forcePackEmptyChar.Bool()
						fmt.Printf("Setting force pack empty char option to: %v\n", options.ForcePackEmptyChar)
					}

					// Set no prev char colors option
					if noPrevCharColors := jsOptions.Get("noPrevCharColors"); noPrevCharColors.Type() == js.TypeBoolean {
						options.NoPrevCharColors = noPrevCharColors.Bool()
						fmt.Printf("Setting no prev char colors option to: %v\n", options.NoPrevCharColors)
					}

					// Set no bitpair counters option
					if noBitpairCounters := jsOptions.Get("noBitpairCounters"); noBitpairCounters.Type() == js.TypeBoolean {
						options.NoBitpairCounters = noBitpairCounters.Bool()
						fmt.Printf("Setting no bitpair counters option to: %v\n", options.NoBitpairCounters)
					}

					// Set no guess option
					if noGuess := jsOptions.Get("noGuess"); noGuess.Type() == js.TypeBoolean {
						options.NoGuess = noGuess.Bool()
						fmt.Printf("Setting no guess option to: %v\n", options.NoGuess)
					}

					// Set alternative fade option
					if alternativeFade := jsOptions.Get("alternativeFade"); alternativeFade.Type() == js.TypeBoolean {
						options.AlternativeFade = alternativeFade.Bool()
						fmt.Printf("Setting alternative fade option to: %v\n", options.AlternativeFade)
					}

					// Set no fade option
					if noFade := jsOptions.Get("noFade"); noFade.Type() == js.TypeBoolean {
						options.NoFade = noFade.Bool()
						fmt.Printf("Setting no fade option to: %v\n", options.NoFade)
					}

					// Set no animation option
					if noAnimation := jsOptions.Get("noAnimation"); noAnimation.Type() == js.TypeBoolean {
						options.NoAnimation = noAnimation.Bool()
						fmt.Printf("Setting no animation option to: %v\n", options.NoAnimation)
					}

					// Set frame delay
					if frameDelay := jsOptions.Get("frameDelay"); frameDelay.Type() == js.TypeNumber {
						options.FrameDelay = byte(frameDelay.Int())
						fmt.Printf("Setting frame delay to: %d\n", options.FrameDelay)
					}

					// Set wait seconds
					if waitSeconds := jsOptions.Get("waitSeconds"); waitSeconds.Type() == js.TypeNumber {
						options.WaitSeconds = waitSeconds.Int()
						fmt.Printf("Setting wait seconds to: %d\n", options.WaitSeconds)
					}

					// Set D016 offset
					if d016Offset := jsOptions.Get("d016Offset"); d016Offset.Type() == js.TypeNumber {
						options.D016Offset = d016Offset.Int()
						fmt.Printf("Setting D016 offset to: %d\n", options.D016Offset)
					}

					// Set no crunch option
					if noCrunch := jsOptions.Get("noCrunch"); noCrunch.Type() == js.TypeBoolean {
						options.NoCrunch = noCrunch.Bool()
						fmt.Printf("Setting no crunch option to: %v\n", options.NoCrunch)
					}

					// Set symbols option
					if symbols := jsOptions.Get("symbols"); symbols.Type() == js.TypeBoolean {
						options.Symbols = symbols.Bool()
						fmt.Printf("Setting symbols option to: %v\n", options.Symbols)
					}

					// Set num workers
					if numWorkers := jsOptions.Get("numWorkers"); numWorkers.Type() == js.TypeNumber {
						options.NumWorkers = numWorkers.Int()
						fmt.Printf("Setting num workers to: %d\n", options.NumWorkers)
					}
				}

				// Convert the image to PRG
				imgReader := bytes.NewReader(bufferBytes)
				converter, err := png2prg.New(options, imgReader)
				if err != nil {
					errMsg := fmt.Sprintf("Failed to process image: %s", err.Error())
					fmt.Println(errMsg)
					// Restore stdout
					w.Close()
					os.Stdout = oldStdout

					// Get the console output
					outputBytes, _ := io.ReadAll(r)

					reject.Invoke(map[string]interface{}{
						"error":         errMsg,
						"consoleOutput": string(outputBytes),
					})
					return nil
				}

				// Write the PRG data to a buffer
				var prgBuffer bytes.Buffer
				_, err = converter.WriteTo(&prgBuffer)
				if err != nil {
					errMsg := fmt.Sprintf("Failed to convert image: %s", err.Error())
					fmt.Println(errMsg)

					// Restore stdout
					w.Close()
					os.Stdout = oldStdout

					// Get the console output
					outputBytes, _ := io.ReadAll(r)

					reject.Invoke(map[string]interface{}{
						"error":         errMsg,
						"consoleOutput": string(outputBytes),
					})
					return nil
				}

				// Flush stdout and restore
				w.Close()
				os.Stdout = oldStdout

				// Get the console output
				outputBytes, _ := io.ReadAll(r)
				consoleOutputStr := string(outputBytes)

				// Get the PRG data
				prgBytes := prgBuffer.Bytes()
				fmt.Printf("Converted to PRG: %d bytes\n", len(prgBytes))

				// Create Uint8Array for the PRG data
				prgArray := js.Global().Get("Uint8Array").New(len(prgBytes))
				js.CopyBytesToJS(prgArray, prgBytes)

				// Create result object with proper initialization
				result := js.ValueOf(map[string]interface{}{
					"data":          prgArray,
					"size":          js.ValueOf(len(prgBytes)),
					"graphicsType":  js.ValueOf(converter.FinalGraphicsType.String()),
					"message":       js.ValueOf("Image successfully converted to PRG"),
					"consoleOutput": js.ValueOf(consoleOutputStr),
				})

				resolve.Invoke(result)
				return nil
			}),
			js.FuncOf(func(this js.Value, errArgs []js.Value) interface{} {
				fmt.Printf("Error reading buffer from proxy %s: %s\n",
					proxyUrl, errArgs[0].Get("message").String())

				// Try the next proxy
				tryNextProxy(proxyUrls, index+1, resolve, reject, jsOptions)
				return nil
			}),
		)
		return nil
	})

	// Handle fetch errors
	processError := js.FuncOf(func(this js.Value, errArgs []js.Value) interface{} {
		fmt.Printf("Fetch error with proxy %s: %s\n",
			proxyUrl, errArgs[0].Get("message").String())

		// Try the next proxy
		tryNextProxy(proxyUrls, index+1, resolve, reject, jsOptions)
		return nil
	})

	// Chain the promise
	fetchPromise.Call("then", processResponse, processError)
}

// convertDirectImageData - converts image data directly provided by JavaScript
func convertDirectImageData(this js.Value, args []js.Value) interface{} {
	// Check if image data is provided
	if len(args) < 1 || !args[0].Truthy() {
		return map[string]interface{}{
			"error": "No image data provided",
		}
	}

	// Get image data from JS
	jsImageData := args[0]

	// Get options from the function arguments
	var jsOptions js.Value
	if len(args) > 1 && args[1].Type() == js.TypeObject {
		jsOptions = args[1]
	}

	fmt.Printf("Received direct image data of length: %d\n", jsImageData.Length())

	// Create a JavaScript Promise
	handler := js.FuncOf(func(this js.Value, handlerArgs []js.Value) interface{} {
		resolve := handlerArgs[0]
		reject := handlerArgs[1]

		// Save original stdout
		oldStdout := os.Stdout
		// Create a pipe to capture output
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Convert JS Uint8Array to Go byte slice
		imageBytes := make([]byte, jsImageData.Length())
		js.CopyBytesToGo(imageBytes, jsImageData)

		// Create options for PNG2PRG
		options := png2prg.Options{
			Display: true,  // Include displayer
			Quiet:   false, // Enable verbose output
		}

		// Apply JavaScript options if provided
		if jsOptions.Truthy() {
			// Set graphics mode if specified
			if mode := jsOptions.Get("mode"); mode.Type() == js.TypeString && mode.String() != "" {
				options.GraphicsMode = mode.String()
				fmt.Printf("Setting graphics mode to: %s\n", options.GraphicsMode)
			}

			// Set bitpair colors if specified
			if bpc := jsOptions.Get("bitpairColors"); bpc.Type() == js.TypeString && bpc.String() != "" {
				options.BitpairColorsString = bpc.String()
				fmt.Printf("Setting bitpair colors to: %s\n", options.BitpairColorsString)
			}

			// Set bitpair colors 2 if specified
			if bpc2 := jsOptions.Get("bitpairColors2"); bpc2.Type() == js.TypeString && bpc2.String() != "" {
				options.BitpairColorsString2 = bpc2.String()
				fmt.Printf("Setting bitpair colors 2 to: %s\n", options.BitpairColorsString2)
			}

			// Set bitpair colors 3 if specified
			if bpc3 := jsOptions.Get("bitpairColors3"); bpc3.Type() == js.TypeString && bpc3.String() != "" {
				options.BitpairColorsString3 = bpc3.String()
				fmt.Printf("Setting bitpair colors 3 to: %s\n", options.BitpairColorsString3)
			}

			// Add all other options as previously implemented in fetchImageData
			if display := jsOptions.Get("display"); display.Type() == js.TypeBoolean {
				options.Display = display.Bool()
				fmt.Printf("Setting display option to: %v\n", options.Display)
			}

			if bf := jsOptions.Get("bruteForce"); bf.Type() == js.TypeBoolean {
				options.BruteForce = bf.Bool()
				fmt.Printf("Setting brute force option to: %v\n", options.BruteForce)
			}

			if interlace := jsOptions.Get("interlace"); interlace.Type() == js.TypeBoolean {
				options.Interlace = interlace.Bool()
				fmt.Printf("Setting interlace option to: %v\n", options.Interlace)
			}

			if forceBorderColor := jsOptions.Get("forceBorderColor"); forceBorderColor.Type() == js.TypeNumber {
				options.ForceBorderColor = forceBorderColor.Int()
				fmt.Printf("Setting force border color to: %d\n", options.ForceBorderColor)
			}

			if forceXOffset := jsOptions.Get("forceXOffset"); forceXOffset.Type() == js.TypeNumber {
				options.ForceXOffset = forceXOffset.Int()
				fmt.Printf("Setting force X offset to: %d\n", options.ForceXOffset)
			}

			if forceYOffset := jsOptions.Get("forceYOffset"); forceYOffset.Type() == js.TypeNumber {
				options.ForceYOffset = forceYOffset.Int()
				fmt.Printf("Setting force Y offset to: %d\n", options.ForceYOffset)
			}

			if noPackChars := jsOptions.Get("noPackChars"); noPackChars.Type() == js.TypeBoolean {
				options.NoPackChars = noPackChars.Bool()
				fmt.Printf("Setting no pack chars option to: %v\n", options.NoPackChars)
			}

			if noPackEmptyChar := jsOptions.Get("noPackEmptyChar"); noPackEmptyChar.Type() == js.TypeBoolean {
				options.NoPackEmptyChar = noPackEmptyChar.Bool()
				fmt.Printf("Setting no pack empty char option to: %v\n", options.NoPackEmptyChar)
			}

			if forcePackEmptyChar := jsOptions.Get("forcePackEmptyChar"); forcePackEmptyChar.Type() == js.TypeBoolean {
				options.ForcePackEmptyChar = forcePackEmptyChar.Bool()
				fmt.Printf("Setting force pack empty char option to: %v\n", options.ForcePackEmptyChar)
			}

			if noPrevCharColors := jsOptions.Get("noPrevCharColors"); noPrevCharColors.Type() == js.TypeBoolean {
				options.NoPrevCharColors = noPrevCharColors.Bool()
				fmt.Printf("Setting no prev char colors option to: %v\n", options.NoPrevCharColors)
			}

			if noBitpairCounters := jsOptions.Get("noBitpairCounters"); noBitpairCounters.Type() == js.TypeBoolean {
				options.NoBitpairCounters = noBitpairCounters.Bool()
				fmt.Printf("Setting no bitpair counters option to: %v\n", options.NoBitpairCounters)
			}

			if noGuess := jsOptions.Get("noGuess"); noGuess.Type() == js.TypeBoolean {
				options.NoGuess = noGuess.Bool()
				fmt.Printf("Setting no guess option to: %v\n", options.NoGuess)
			}

			if alternativeFade := jsOptions.Get("alternativeFade"); alternativeFade.Type() == js.TypeBoolean {
				options.AlternativeFade = alternativeFade.Bool()
				fmt.Printf("Setting alternative fade option to: %v\n", options.AlternativeFade)
			}

			if noFade := jsOptions.Get("noFade"); noFade.Type() == js.TypeBoolean {
				options.NoFade = noFade.Bool()
				fmt.Printf("Setting no fade option to: %v\n", options.NoFade)
			}

			if noAnimation := jsOptions.Get("noAnimation"); noAnimation.Type() == js.TypeBoolean {
				options.NoAnimation = noAnimation.Bool()
				fmt.Printf("Setting no animation option to: %v\n", options.NoAnimation)
			}

			if frameDelay := jsOptions.Get("frameDelay"); frameDelay.Type() == js.TypeNumber {
				options.FrameDelay = byte(frameDelay.Int())
				fmt.Printf("Setting frame delay to: %d\n", options.FrameDelay)
			}

			if waitSeconds := jsOptions.Get("waitSeconds"); waitSeconds.Type() == js.TypeNumber {
				options.WaitSeconds = waitSeconds.Int()
				fmt.Printf("Setting wait seconds to: %d\n", options.WaitSeconds)
			}

			if d016Offset := jsOptions.Get("d016Offset"); d016Offset.Type() == js.TypeNumber {
				options.D016Offset = d016Offset.Int()
				fmt.Printf("Setting D016 offset to: %d\n", options.D016Offset)
			}

			if noCrunch := jsOptions.Get("noCrunch"); noCrunch.Type() == js.TypeBoolean {
				options.NoCrunch = noCrunch.Bool()
				fmt.Printf("Setting no crunch option to: %v\n", options.NoCrunch)
			}

			if symbols := jsOptions.Get("symbols"); symbols.Type() == js.TypeBoolean {
				options.Symbols = symbols.Bool()
				fmt.Printf("Setting symbols option to: %v\n", options.Symbols)
			}

			if numWorkers := jsOptions.Get("numWorkers"); numWorkers.Type() == js.TypeNumber {
				options.NumWorkers = numWorkers.Int()
				fmt.Printf("Setting num workers to: %d\n", options.NumWorkers)
			}
		}

		// Convert the image to PRG
		imgReader := bytes.NewReader(imageBytes)
		converter, err := png2prg.New(options, imgReader)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to process image: %s", err.Error())
			fmt.Println(errMsg)

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Get the console output
			outputBytes, _ := io.ReadAll(r)

			reject.Invoke(map[string]interface{}{
				"error":         errMsg,
				"consoleOutput": string(outputBytes),
			})
			return nil
		}

		// Write the PRG data to a buffer
		var prgBuffer bytes.Buffer
		_, err = converter.WriteTo(&prgBuffer)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to convert image: %s", err.Error())
			fmt.Println(errMsg)

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Get the console output
			outputBytes, _ := io.ReadAll(r)

			reject.Invoke(map[string]interface{}{
				"error":         errMsg,
				"consoleOutput": string(outputBytes),
			})
			return nil
		}

		// Flush stdout and restore
		w.Close()
		os.Stdout = oldStdout

		// Get the console output
		outputBytes, _ := io.ReadAll(r)
		consoleOutputStr := string(outputBytes)

		// Get the PRG data
		prgBytes := prgBuffer.Bytes()
		fmt.Printf("Converted to PRG: %d bytes\n", len(prgBytes))

		// Create Uint8Array for the PRG data
		prgArray := js.Global().Get("Uint8Array").New(len(prgBytes))
		js.CopyBytesToJS(prgArray, prgBytes)

		// Create result object with proper initialization
		result := js.ValueOf(map[string]interface{}{
			"data":          prgArray,
			"size":          js.ValueOf(len(prgBytes)),
			"graphicsType":  js.ValueOf(converter.FinalGraphicsType.String()),
			"message":       js.ValueOf("Image successfully converted to PRG"),
			"consoleOutput": js.ValueOf(consoleOutputStr),
		})

		// Return the result
		resolve.Invoke(result)
		return nil
	})

	// Return a new Promise to JavaScript
	return js.Global().Get("Promise").New(handler)
}

// Helper function to calculate the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
