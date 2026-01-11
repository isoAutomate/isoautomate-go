package main

import (
	"fmt"
	"log"
	"time"

	"github.com/isoAutomate/isoautomate-go" // Adjust if your module name is different
)

func main() {
	// 1. Configure
	cfg := isoautomate.Config{
		RedisHost: "localhost", // Change to your Redis host
		RedisPort: "6379",
	}

	// 2. Connect
	client, err := isoautomate.New(cfg)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	fmt.Println("Connected to Redis! Acquiring browser...")

	// 3. Acquire (Chrome, No Video, No Profile, No Record)
	// Passing 'nil' for profile means standard ephemeral session
	_, err = client.Acquire("chrome", false, nil, false)
	if err != nil {
		log.Fatalf("Failed to acquire: %v", err)
	}
	// Ensure we release at the end
	defer client.Release()

	// 4. Run Actions
	fmt.Println("Browser acquired! Navigating...")

	_, _ = client.OpenURL("https://google.com")

	titleRes, _ := client.GetTitle()
	fmt.Printf("Page Title: %v\n", titleRes["value"])

	fmt.Println("Taking screenshot...")
	pathRes, _ := client.Screenshot("example.png", "")
	fmt.Printf("Screenshot saved to: %v\n", pathRes["path"])

	time.Sleep(2 * time.Second)
	fmt.Println("Done. Releasing...")
}
