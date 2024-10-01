package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

var (
	templates *template.Template
	services  map[string]Service
)

type Service struct {
	Name     string    `json:"name"`
	Endpoint string    `json:"endpoint"`
	Children []Service `json:"children"`
}

type Product struct {
	ID              int    `json:"id"`
	Title           string `json:"title"`
	DescriptionHtml string `json:"descriptionHtml"`
}

func init() {
	// Carregar os templates
	templates = template.Must(template.ParseGlob("templates/*.html"))

	// Configurar os serviços
	services = setupServices()
}

func main() {
	port := "8083"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}

	r := mux.NewRouter()

	r.HandleFunc("/", indexHandler).Methods("GET")
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/productpage", productPageHandler).Methods("GET")
	r.HandleFunc("/api/v1/products", productsHandler).Methods("GET")
	r.HandleFunc("/api/v1/products/{id}", productHandler).Methods("GET")
	r.HandleFunc("/api/v1/products/{id}/reviews", productReviewsHandler).Methods("GET")
	r.HandleFunc("/api/v1/products/{id}/ratings", productRatingsHandler).Methods("GET")

	r.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static")))).Methods("GET")

	http.Handle("/", r)

	log.Printf("Server started at :%s\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func jsonToHTMLTable(data Service) string {
	html := "<table class='table table-condensed table-bordered table-hover'>"
	html += "<thead><tr><th>Name</th><th>Endpoint</th><th>Children</th></tr></thead>"
	html += "<tbody>"
	html += buildHTMLTableRow(data)
	html += "</tbody></table>"
	return html
}

func buildHTMLTableRow(service Service) string {
	row := "<tr>"
	row += fmt.Sprintf("<td>%s</td>", service.Name)
	row += fmt.Sprintf("<td>%s</td>", service.Endpoint)

	// Verifica se o serviço tem filhos
	if len(service.Children) > 0 {
		row += "<td><table>"
		for _, child := range service.Children {
			row += buildHTMLTableRow(child)
		}
		row += "</table></td>"
	} else {
		row += "<td>None</td>"
	}
	row += "</tr>"
	return row
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	productPage := services["productpage"]

	// Convertendo o JSON para uma tabela HTML
	table := jsonToHTMLTable(productPage)

	// Definindo o tipo de conteúdo como HTML
	w.Header().Set("Content-Type", "text/html")

	if err := templates.ExecuteTemplate(w, "index.html", map[string]interface{}{
		"serviceTable": template.HTML(table),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Product page is healthy")
}

// makeSeq gera uma sequência de números de 0 até n-1
func makeSeq(n int) []int {
	seq := make([]int, n)
	for i := 0; i < n; i++ {
		seq[i] = i
	}
	return seq
}

func productPageHandler(w http.ResponseWriter, r *http.Request) {
	productID := 0 // valor padrão

	headers := forwardHeaders(r)
	products := getProducts()
	product := products[0]

	detailsStatus, details := getProductDetails(productID, headers)

	reviewsStatus, reviews := getProductReviews(productID, headers)

	fmt.Println(product.DescriptionHtml)

	// Exemplo de valor de rating
	stars := 4 // Substitua isso com o valor real de estrelas das avaliações

	// Preparando os dados para passar ao template
	data := map[string]interface{}{
		"detailsStatus": detailsStatus,
		"reviewsStatus": reviewsStatus,
		"product":       product,
		"details":       details,
		"reviews":       reviews,
		"user":          "", // Usuário não implementado
		"rating": map[string]interface{}{
			"stars": makeSeq(stars), // Gera a sequência de estrelas
			"color": "yellow",
		},
	}

	w.Header().Set("Content-Type", "text/html")

	if err := templates.ExecuteTemplate(w, "productpage.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Funções auxiliares
func setupServices() map[string]Service {
	servicesDomain := getEnv("SERVICES_DOMAIN", "")
	detailsHostname := getEnv("DETAILS_HOSTNAME", "localhost")
	detailsPort := getEnv("DETAILS_SERVICE_PORT", "9084")
	ratingsHostname := getEnv("RATINGS_HOSTNAME", "localhost")
	ratingsPort := getEnv("RATINGS_SERVICE_PORT", "8085")
	reviewsHostname := getEnv("REVIEWS_HOSTNAME", "localhost")
	reviewsPort := getEnv("REVIEWS_SERVICE_PORT", "9086")

	details := Service{
		Name:     fmt.Sprintf("http://%s%s:%s", detailsHostname, servicesDomain, detailsPort),
		Endpoint: "details",
		Children: []Service{},
	}

	ratings := Service{
		Name:     fmt.Sprintf("http://%s%s:%s", ratingsHostname, servicesDomain, ratingsPort),
		Endpoint: "ratings",
	}

	reviews := Service{
		Name:     fmt.Sprintf("http://%s%s:%s", reviewsHostname, servicesDomain, reviewsPort),
		Endpoint: "reviews",
		Children: []Service{ratings},
	}

	productPage := Service{
		Name:     fmt.Sprintf("http://%s%s:%s", detailsHostname, servicesDomain, detailsPort),
		Endpoint: "details",
		Children: []Service{details, reviews},
	}

	return map[string]Service{
		"productpage": productPage,
		"details":     details,
		"reviews":     reviews,
		"ratings":     ratings,
	}
}

func toHTMLTable(service Service) string {
	// Converte o serviço em tabela HTML
	data, _ := json.Marshal(service)
	return string(data)
}

func forwardHeaders(r *http.Request) map[string]string {
	headers := map[string]string{}
	// Aqui ficariam as lógicas de propagação de cabeçalhos
	return headers
}

func productsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	products := getProducts()

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	err := enc.Encode(products)
	if err != nil {
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
		return
	}

	w.Write(buf.Bytes())
}

func productHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]

	detailsService := services["details"]                               // Pega o serviço "details" do mapa de serviços
	url := fmt.Sprintf("%s/details/%s", detailsService.Name, productID) // Constrói a URL com o ID do produto

	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, "Failed to fetch product details", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Lê a resposta do serviço "details"
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response from details service", http.StatusInternalServerError)
		return
	}

	// Escreve a resposta JSON do serviço "details" para o cliente
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func productReviewsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]

	reviewsService := services["reviews"]                               // Pega o serviço "reviews" do mapa de serviços
	url := fmt.Sprintf("%s/reviews/%s", reviewsService.Name, productID) // Constrói a URL com o ID do produto

	fmt.Println(reviewsService)

	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, "Failed to fetch product reviews", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Lê a resposta do serviço "reviews"
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response from reviews service", http.StatusInternalServerError)
		return
	}

	// Escreve a resposta JSON do serviço "reviews" para o cliente
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func productRatingsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]

	ratingsService := services["ratings"]                               // Pega o serviço "ratings" do mapa de serviços
	url := fmt.Sprintf("%s/ratings/%s", ratingsService.Name, productID) // Constrói a URL com o ID do produto

	fmt.Println(ratingsService)

	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, "Failed to fetch product ratings", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Lê a resposta do serviço "ratings"
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response from ratings service", http.StatusInternalServerError)
		return
	}

	// Escreve a resposta JSON do serviço "ratings" para o cliente
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func getProducts() []Product {
	return []Product{
		{
			ID:              0,
			Title:           "The Comedy of Errors",
			DescriptionHtml: "<a href=\"https://en.wikipedia.org/wiki/The_Comedy_of_Errors\">Wikipedia Summary</a>: The Comedy of Errors is one of <b>William Shakespeare's</b> early plays. It is his shortest and one of his most farcical comedies, with a major part of the humour coming from slapstick and mistaken identity, in addition to puns and word play.",
		},
	}
}

func getProductDetails(productID int, headers map[string]string) (int, map[string]interface{}) {
	// Constroi a URL para o serviço details
	detailsService := services["details"]                               // Pega o serviço "ratings" do mapa de serviços
	url := fmt.Sprintf("%s/details/%d", detailsService.Name, productID) // Constrói a URL com o ID do produto
	// url := fmt.Sprintf("http://localhost:9084/details/%d", productID)

	fmt.Println(url)

	// Cria uma nova requisição HTTP GET
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return 500, nil
	}

	// Adiciona os headers na requisição, se houver
	for key, value := range headers {
		req.Header.Add(key, value)
	}

	// Executa a requisição
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return 500, nil
	}
	defer resp.Body.Close()

	// Lê o corpo da resposta
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return 500, nil
	}

	// Verifica se a requisição foi bem-sucedida
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Received non-200 response: %d\n", resp.StatusCode)
		return resp.StatusCode, nil
	}

	// Decodifica a resposta JSON para um mapa
	var details map[string]interface{}
	err = json.Unmarshal(body, &details)
	if err != nil {
		fmt.Println("Error unmarshalling response:", err)
		return 500, nil
	}

	// Retorna o status e os detalhes do produto
	return resp.StatusCode, details
}

func getProductReviews(productID int, headers map[string]string) (int, map[string]interface{}) {
	return 200, map[string]interface{}{
		"reviews": "Product reviews",
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
