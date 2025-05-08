package main

import (
	"bytes"
	"fmt"
	"net/url"
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

	// Inside tryNextProxy function, before creating processResponse
	fmt.Printf("Fetching from proxy: %s\n", proxyUrl)

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
				}

				// Convert the image to PRG
				imgReader := bytes.NewReader(bufferBytes)
				converter, err := png2prg.New(options, imgReader)
				if err != nil {
					errMsg := fmt.Sprintf("Failed to process image: %s", err.Error())
					fmt.Println(errMsg)
					reject.Invoke(errMsg)
					return nil
				}

				// Write the PRG data to a buffer
				var prgBuffer bytes.Buffer
				_, err = converter.WriteTo(&prgBuffer)
				if err != nil {
					errMsg := fmt.Sprintf("Failed to convert image: %s", err.Error())
					fmt.Println(errMsg)
					reject.Invoke(errMsg)
					return nil
				}

				// Get the PRG data
				prgBytes := prgBuffer.Bytes()
				fmt.Printf("Converted to PRG: %d bytes\n", len(prgBytes))

				// Create Uint8Array for the PRG data
				prgArray := js.Global().Get("Uint8Array").New(len(prgBytes))
				js.CopyBytesToJS(prgArray, prgBytes)

				// Print additional debug information to help diagnose the issue
				fmt.Printf("DEBUG: PRG array length in JS: %d\n", prgArray.Get("length").Int())
				fmt.Printf("DEBUG: PRG array type: %s\n", prgArray.Get("constructor").Get("name").String())

				// Create result object with proper initialization
				result := js.ValueOf(map[string]interface{}{
					"data":         prgArray,
					"size":         js.ValueOf(len(prgBytes)),
					"graphicsType": js.ValueOf(converter.FinalGraphicsType.String()),
					"message":      js.ValueOf("Image successfully converted to PRG"),
				})

				// Inside your Go code, right before invoking the resolve function:
				fmt.Printf("Sending result to JavaScript: %+v\n", result)
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
		}

		// Convert the image to PRG
		imgReader := bytes.NewReader(imageBytes)
		converter, err := png2prg.New(options, imgReader)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to process image: %s", err.Error())
			fmt.Println(errMsg)
			reject.Invoke(errMsg)
			return nil
		}

		// Write the PRG data to a buffer
		var prgBuffer bytes.Buffer
		_, err = converter.WriteTo(&prgBuffer)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to convert image: %s", err.Error())
			fmt.Println(errMsg)
			reject.Invoke(errMsg)
			return nil
		}

		// Get the PRG data
		prgBytes := prgBuffer.Bytes()
		fmt.Printf("Converted to PRG: %d bytes\n", len(prgBytes))

		// Create Uint8Array for the PRG data
		prgArray := js.Global().Get("Uint8Array").New(len(prgBytes))
		js.CopyBytesToJS(prgArray, prgBytes)

		// Print additional debug information to help diagnose the issue
		fmt.Printf("DEBUG: PRG array length in JS: %d\n", prgArray.Get("length").Int())
		fmt.Printf("DEBUG: PRG array type: %s\n", prgArray.Get("constructor").Get("name").String())

		// Create result object with proper initialization
		result := js.ValueOf(map[string]interface{}{
			"data":         prgArray,
			"size":         js.ValueOf(len(prgBytes)),
			"graphicsType": js.ValueOf(converter.FinalGraphicsType.String()),
			"message":      js.ValueOf("Image successfully converted to PRG"),
		})

		// Inside your Go code, right before invoking the resolve function:
		fmt.Printf("Sending result to JavaScript: %+v\n", result)
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
