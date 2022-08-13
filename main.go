package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

type RestaurantInfo struct {
	name    string
	address string
	website string
	email   string
	phone   string
	url     string
}

func (restaurantInfo RestaurantInfo) ToSlice() []string {
	var res []string = []string{
		restaurantInfo.name,
		restaurantInfo.address,
		restaurantInfo.website,
		restaurantInfo.email,
		restaurantInfo.phone,
		restaurantInfo.url,
	}

	return res
}

var baseUrl string = "https://www.tripadvisor.com"

// Variables
var cityGroupsCounter int = 2
var restaurantsCounters = make(map[string]int)

// Constraints
var maxRegionsDepth int = 1      // max number of URLs read from the file region_urls.txt or -1 for no limit
var maxCityGroupsDepth int = -1  // number of pages read or -1 for no limit
var maxRestaurantsDepth int = -1 // number of pages read or -1 for no limit

// Read the region URLs
// from a configuration file
func ReadURLsFile(path string) []string {
	// open file
	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("There was an error opening the file \"%s\".\nError: \"%s\"\n", path, err)
	}
	// close the file at the end of the program
	defer f.Close()

	urls := make([]string, 0)

	// read the file line by line using scanner
	scanner := bufio.NewScanner(f)

	// Read all the URLs or limit the reads based on the constraint maxRegionsDepth
	var i int = 0
	for scanner.Scan() {
		i++
		var row string = scanner.Text()
		urls = append(urls, row)
		if i >= maxRegionsDepth && maxRegionsDepth > 0 {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return urls
}

// Get the list of restaurants groups URLs from the region URLs
// read from the input slice
func getListByRegion(urls []string) []string {

	var firstPageURLs []string
	var secondPage string

	var cityGroupURLs []string

	for _, url := range urls {

		// save the region URL into a file
		var groupsFilename string = "region_group_urls.txt"

		file, errOpen := os.OpenFile(groupsFilename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if errOpen != nil {
			fmt.Printf("There was an error opening the file \"%v\".\nError: \"%s\"\n", file, errOpen)
			os.Exit(2)
		}
		_, errWrite := file.WriteString(url + "\n")
		if errWrite != nil {
			fmt.Printf("There was an error writing in the file \"%v\".\nError: \"%s\"\n", file, errWrite)
		}

		// get first page groups URLs and second page base URL
		firstPageURLs, secondPage = GetListByRegionFirstPage(url)

		// save the groups URLs
		cityGroupURLs = append(cityGroupURLs, firstPageURLs...)

		// get following pages city groups starting from the second page (recursive function)
		if maxCityGroupsDepth > 1 || maxCityGroupsDepth < 0 { // read the second page only if the depth constraint is bigger than 1 or there is no limit
			returnUrls, _ := GetListByRegionFollowingPage(secondPage)
			cityGroupURLs = append(cityGroupURLs, returnUrls...)
		}

		// Save the groups into a file after the region URL
		for _, cityGroupURL := range cityGroupURLs {
			_, errWrite := file.WriteString("\t" + cityGroupURL + "\n")
			if errWrite != nil {
				fmt.Printf("There was an error writing in the file \"%v\".\nError: \"%s\"\n", file, errWrite)
			}
		}

		defer file.Close()
	}

	return cityGroupURLs
}

func GetListByRegionFirstPage(url string) ([]string, string) {
	returnUrls := make([]string, 0)
	var secondPage string
	c := colly.NewCollector(
		colly.AllowedDomains(
			"https://www.tripadvisor.com/",
			"https://www.tripadvisor.com",
			"www.tripadvisor.com/",
			"www.tripadvisor.com",
			"tripadvisor.com/",
			"tripadvisor.com",
		),
		colly.Async(),
		colly.UserAgent("xy"),
		colly.AllowURLRevisit(),
	)
	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 5})

	// Get the region restaurants groups from the first page
	c.OnHTML("#BROAD_GRID .geo_name", func(e *colly.HTMLElement) {
		var urls []string = e.ChildAttrs("a", "href")
		for _, url := range urls {
			returnUrls = append(returnUrls, baseUrl+url)
		}
	})

	// Read the second page
	c.OnHTML(".pageNumbers a[data-page-number=\"2\"]", func(e *colly.HTMLElement) {
		secondPage = baseUrl + e.Attr("href")
	})

	c.OnResponse(func(r *colly.Response) {
		// fmt.Printf("\tDone visiting %v\n", r.Request.URL)
	})

	c.OnRequest(func(r *colly.Request) {
		// fmt.Printf("\tVisiting: %v\n", r.URL)
	})

	c.Visit(url)

	c.Wait()
	return returnUrls, secondPage
}

func GetListByRegionFollowingPage(url string) ([]string, string) {

	returnUrls := make([]string, 0)
	var nextPage string
	c := colly.NewCollector(
		colly.AllowedDomains(
			"https://www.tripadvisor.com/",
			"https://www.tripadvisor.com",
			"www.tripadvisor.com/",
			"www.tripadvisor.com",
			"tripadvisor.com/",
			"tripadvisor.com",
		),
		colly.Async(),
		colly.UserAgent("xy"),
		colly.AllowURLRevisit(),
	)
	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 5})

	// Get the region restaurants groups from the page
	c.OnHTML(".geoList li", func(e *colly.HTMLElement) {
		var urls []string = e.ChildAttrs("a", "href")
		for _, url := range urls {
			returnUrls = append(returnUrls, baseUrl+url)
		}
	})

	// Read the next page
	c.OnHTML(".deckTools.btm .pgLinks a.sprite-pageNext", func(e *colly.HTMLElement) {
		nextPage = baseUrl + e.Attr("href")
		if nextPage != "" {
			if maxCityGroupsDepth < 0 {
				// cycle through all the pages
			} else {
				// limit the calls
				if cityGroupsCounter >= maxCityGroupsDepth {
					// depth limit constraint reached, exit
					return
				}
			}
			cityGroupsCounter++
			// recursively call the same method
			returnUrlsNext, _ := GetListByRegionFollowingPage(nextPage)
			returnUrls = append(returnUrls, returnUrlsNext...)
		}

	})

	c.OnResponse(func(r *colly.Response) {
		// fmt.Printf("\tDone visiting %v\n", r.Request.URL)
	})

	c.OnRequest(func(r *colly.Request) {
		// fmt.Printf("\tVisiting: %v\n", r.URL)
	})

	c.Visit(url)

	c.Wait()
	return returnUrls, nextPage
}

func GetRestaurantInfo(url string) RestaurantInfo {
	var res RestaurantInfo

	res.url = url

	c := colly.NewCollector(
		colly.AllowedDomains(
			"https://www.tripadvisor.com/",
			"https://www.tripadvisor.com",
			"www.tripadvisor.com/",
			"www.tripadvisor.com",
			"tripadvisor.com/",
			"tripadvisor.com",
		),
		colly.Async(),
		colly.UserAgent("xy"),
		colly.AllowURLRevisit(),
	)

	c.Limit(&colly.LimitRule{
		// Filter domains affected by this rule
		DomainGlob: "*",
		// Set a delay between requests to these domains
		Delay: 1 * time.Second,
		// Add an additional random delay
		RandomDelay: 1 * time.Second,
		Parallelism: 5,
	})

	// Restaurant name
	c.OnHTML(".acKDw h1.HjBfq", func(e *colly.HTMLElement) {
		// fmt.Println("name:", e.Text)
		res.name = e.Text
	})
	// Restaurant address
	c.OnHTML(".xLvvm:nth-of-type(3) .kDZhm:nth-of-type(1) span:nth-of-type(2) a .yEWoV", func(e *colly.HTMLElement) {
		// fmt.Println("address:", e.Text)
		res.address = e.Text
	})
	// Restaurant website
	c.OnHTML(".xLvvm:nth-of-type(3)", func(e *colly.HTMLElement) {
		qoquerySelection := e.DOM
		encodedUrl, found := qoquerySelection.Find(".f").Find(".f").Find(".YnKZo").Attr("data-encoded-url")
		var url [][]byte
		var website string = ""
		if found {
			decodedUrl, err := base64.StdEncoding.DecodeString(encodedUrl)
			if err != nil {
				panic(err)
			}
			url = bytes.Split(decodedUrl, []byte("_"))
			website = string(url[1])
		}
		res.website = website
	})

	// Restaurant email
	c.OnHTML(".xLvvm:nth-of-type(3) .f .f .IdiaP:nth-of-type(2)", func(e *colly.HTMLElement) {
		var fullEmail string = e.ChildAttr("a", "href")
		email := strings.Replace(fullEmail, "mailto:", "", -1)
		email = strings.Replace(email, "?subject=?", "", -1)
		res.email = email
	})

	// Restaurant phone
	c.OnHTML(".xLvvm:nth-of-type(3) .f:nth-of-type(4)", func(e *colly.HTMLElement) {
		res.phone = e.Text
	})

	c.OnResponse(func(r *colly.Response) {
		fmt.Printf("\tDone visiting %v\n\n", r.Request.URL)
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Printf("\tVisiting: %v\n\n", r.URL)
	})
	c.Visit(url)

	c.Wait()

	return res
}

func getRestaurantsURLsByCityGroup(url string) ([]string, string) {

	// fmt.Printf("\n\ngetRestaurantsURLsByCityGroup: %v\n", url)

	returnUrls := make([]string, 0)
	var nextPage string
	var groupHeading string

	c := colly.NewCollector(
		colly.AllowedDomains(
			"https://www.tripadvisor.com/",
			"https://www.tripadvisor.com",
			"www.tripadvisor.com/",
			"www.tripadvisor.com",
			"tripadvisor.com/",
			"tripadvisor.com",
		),
		colly.Async(),
		colly.UserAgent("xy"),
		colly.AllowURLRevisit(),
	)
	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 5})

	// Get the page heading
	c.OnHTML("#HEADING", func(e *colly.HTMLElement) {
		groupHeading = strings.Trim(e.Text, "\n")
	})

	// Get the restaurants URLs from the page
	c.OnHTML(".YtrWs[data-test-target=\"restaurants-list\"] .YHnoF.Gi.o[data-test!=\"SL_list_item\"] .RfBGI", func(e *colly.HTMLElement) {
		var urls []string = e.ChildAttrs("a", "href")

		for _, url := range urls {
			returnUrls = append(returnUrls, baseUrl+url)
		}

		// fmt.Printf("\n\nRESTAURANTS URLs (for getRestaurantsURLsByCityGroup: %v):\n%v\n\n", url, returnUrls)

	})

	// Read the next page
	c.OnHTML(".pagination .nav.next", func(e *colly.HTMLElement) {
		nextPage = baseUrl + e.Attr("href")

		if nextPage != "" {

			// fmt.Printf("\nNEXT PAGE:\n%v\n", nextPage)
			// fmt.Printf("\trestaurantsCounter[%v]: %v\n\tmaxRestaurantsDepth: %v\n\tgroupHeading: %v\n\n", groupHeading, restaurantsCounters[groupHeading], maxRestaurantsDepth, groupHeading)

			if maxRestaurantsDepth < 0 {
				// cycle through all the pages
			} else {
				// limit the calls
				if restaurantsCounters[groupHeading] >= (maxRestaurantsDepth - 1) {
					// depth limit constraint reached, exit
					return
				}
			}

			restaurantsCounters[groupHeading]++
			// recursively call the same method
			returnUrlsNext, _ := getRestaurantsURLsByCityGroup(nextPage)
			returnUrls = append(returnUrls, returnUrlsNext...)
		}

	})

	c.OnResponse(func(r *colly.Response) {
		// fmt.Printf("\tDone visiting %v\n", r.Request.URL)
	})

	c.OnRequest(func(r *colly.Request) {
		// fmt.Printf("\tVisiting: %v\n", r.URL)
	})

	c.Visit(url)

	c.Wait()
	return returnUrls, nextPage
}

func main() {
	// Save starting time
	start := time.Now()

	var filename string = "region_urls.txt"
	var regionURLs []string = ReadURLsFile(filename)
	totalRegionsRead := len(regionURLs)

	// get all city groups URLs
	var cityGroupURLs []string = getListByRegion(regionURLs)
	totalGroupsRead := len(cityGroupURLs)

	// save the region URL into a file
	var restaurantsFilename string = "region_group_restaurant_urls.txt"

	file, errOpen := os.OpenFile(restaurantsFilename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if errOpen != nil {
		fmt.Printf("There was an error opening the file \"%v\".\nError: \"%s\"\n", file, errOpen)
		os.Exit(2)
	}
	var restaurantsURLs []string
	// get all the restaurants URLs
	for _, cityGroupURL := range cityGroupURLs {

		_, errWrite := file.WriteString(cityGroupURL + "\n")
		if errWrite != nil {
			fmt.Printf("There was an error writing in the file \"%v\".\nError: \"%s\"\n", file, errWrite)
		}

		restaurantsURLs, _ = getRestaurantsURLsByCityGroup(cityGroupURL)

		var restaurantsInfo []RestaurantInfo
		// save the restaurants info into a file
		var restaurantsInfoFilename string = "restaurants_info.csv"
		restaurantsInfoFile, errOpen := os.OpenFile(restaurantsInfoFilename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if errOpen != nil {
			fmt.Printf("There was an error opening the file \"%v\".\nError: \"%s\"\n", restaurantsInfoFile, errOpen)
			os.Exit(3)
		}
		var restaurantsInfoHeaders []string = []string{"name", "address", "website", "email", "phone", "url"}
		restaurantsInfoWriter := csv.NewWriter(restaurantsInfoFile)
		errRestaurantsInfoWrite := restaurantsInfoWriter.Write(restaurantsInfoHeaders)

		if errRestaurantsInfoWrite != nil {
			fmt.Printf("There was an error writing in the file \"%v\".\nError: \"%s\"\n", restaurantsInfoFile, errRestaurantsInfoWrite)
		}

		// Save the restaurants into a file after the group URL
		for _, restaurantsURL := range restaurantsURLs {
			_, errWrite := file.WriteString("\t" + restaurantsURL + "\n")
			if errWrite != nil {
				fmt.Printf("There was an error writing in the file \"%v\".\nError: \"%s\"\n", file, errWrite)
			}

			// Read the restaurant info
			var restaurantInfo RestaurantInfo = GetRestaurantInfo(restaurantsURL)
			restaurantsInfo = append(restaurantsInfo, restaurantInfo)
			errRestaurantsInfoWrite = restaurantsInfoWriter.Write(restaurantInfo.ToSlice())
			if errRestaurantsInfoWrite != nil {
				fmt.Printf("There was an error writing in the file \"%v\".\nError: \"%s\"\n", restaurantsInfoFile, errRestaurantsInfoWrite)
			}

		}
		defer restaurantsInfoWriter.Flush()
	}

	totalRestaurantsRead := len(restaurantsURLs)

	// fmt.Printf("result:\n\n\n%+v\n\n\n", result)

	// Print elapsed time
	fmt.Println("------------------------")
	fmt.Println("Elapsed time:", time.Since(start))
	fmt.Println("Regions read:", totalRegionsRead)
	fmt.Println("Groups read:", totalGroupsRead)
	fmt.Println("Restaurants read:", totalRestaurantsRead)
	fmt.Println("------------------------")
}
