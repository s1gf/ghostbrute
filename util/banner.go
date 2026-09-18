package util

import "fmt"

func PrintBanner() {
	banner := `
          __               __  __               __
   ____ _/ /_  ____  _____/ /_/ /_  _______  __/ /____
  / __ '/ __ \/ __ \/ ___/ __/ __ \/ ___/ / / / __/ _ \
 / /_/ / / / / /_/ (__  ) /_/ /_/ / /  / /_/ / /_/  __/
 \__, /_/ /_/\____/____/\__/_.___/_/   \__,_/\__/\___/
/____/
`
	fmt.Printf("%v\nVersion: %v (%v) - %v - %v\n\n", banner, Version, GitCommit, BuildDate, Author)
}
