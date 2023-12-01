package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Account struct {
	Username string `json:"nombre"`
	Password string `json:"contrasenia"`
}

func main() {

	// Maneja la Página de login
	executeLoginPage()

	// Manejador para servir mainPage.html
	executedMainPage()

	fmt.Println("Servidor web en ejecución en el puerto 8080...")
	http.ListenAndServe(":8080", nil)
}

func executedMainPage() {
	http.HandleFunc("/mainPage", func(w http.ResponseWriter, r *http.Request) {

		// HTML de la página de inicio de sesión con Bootstrap
		fmt.Fprintln(w, "<html><head>")
		fmt.Fprintln(w, `<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.5.2/css/bootstrap.min.css">`)
		fmt.Fprintln(w, "<script src='https://maxcdn.bootstrapcdn.com/bootstrap/4.5.2/js/bootstrap.min.js'></script>")
		fmt.Fprintln(w, "</head><body>")
		fmt.Fprintln(w, "<div class='container mt-5'>")
		fmt.Fprintln(w, "<h1 class='mb-4'>HOLA MUNDO</h1>")
		fmt.Fprintln(w, "</div>")
		fmt.Fprintln(w, "</body></html>")
	})
}

func executeLoginPage() {
	http.HandleFunc("/loginPage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Obtener datos del formulario
			Username := r.FormValue("Username")
			password := r.FormValue("password")

			// Verificar si ambos campos son obligatorios
			if Username == "" || password == "" {
				http.Error(w, "Correo electrónico y contraseña son obligatorios", http.StatusBadRequest)
				return
			}

			// Crear una instancia de Account
			account := Account{
				Username: Username,
				Password: password,
			}

			go saveAccount(account, w, r)
		}

		// HTML de la página de inicio de sesión con Bootstrap
		fmt.Fprintln(w, "<html><head>")
		fmt.Fprintln(w, `<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.5.2/css/bootstrap.min.css">`)
		fmt.Fprintln(w, "<script src='https://maxcdn.bootstrapcdn.com/bootstrap/4.5.2/js/bootstrap.min.js'></script>")
		fmt.Fprintln(w, "</head><body>")
		fmt.Fprintln(w, "<div class='container mt-5'>")
		fmt.Fprintln(w, "<h1 class='mb-4'>Iniciar Sesión</h1>")
		fmt.Fprintln(w, `<form method="POST">`)
		fmt.Fprintln(w, `<div class="form-group">`)
		fmt.Fprintln(w, `<label for="Username">Nombre de Usuario:</label>`)
		fmt.Fprintln(w, `<input type="text" class="form-control" name="Username" required>`)
		fmt.Fprintln(w, `</div>`)
		fmt.Fprintln(w, `<div class="form-group">`)
		fmt.Fprintln(w, `<label for="password">Contraseña:</label>`)
		fmt.Fprintln(w, `<input type="password" class="form-control" name="password" required>`)
		fmt.Fprintln(w, `</div>`)
		fmt.Fprintln(w, `<button type="submit" class="btn btn-primary">Iniciar Sesión</button>`)
		fmt.Fprintln(w, `</form>`)
		fmt.Fprintln(w, "</div>")
		fmt.Fprintln(w, "</body></html>")
	})
}

func saveAccount(account Account, w http.ResponseWriter, r *http.Request) {
	// Codificar la cuenta como JSON
	requestBody, err := json.Marshal(account)
	if err != nil {
		http.Error(w, "Error al codificar JSON", http.StatusInternalServerError)
		return
	}

	// Realizar una solicitud POST al servidor en la ruta "/json/login"
	resp, err := http.Post("http://localhost:8081/json/login", "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		http.Error(w, "Error al enviar solicitud de inicio de sesión", http.StatusInternalServerError)
		return
	}

	// Manejar la respuesta JSON
	var response map[string]bool
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&response); err != nil {
		fmt.Println("Error al decodificar la respuesta JSON:", err)
		return
	}

	defer resp.Body.Close()

	// Verificar si el inicio de sesión fue correcto
	if response["loginCorrecto"] {
		fmt.Println("Inicio de sesión correcto, redirigir al usuario a la página principal")
		http.Redirect(w, r, "http://localhost:8080/mainPage", http.StatusSeeOther)
		// Realizar la redirección a la página principal o realizar otras acciones según sea necesario
	} else {
		fmt.Println("Inicio de sesión incorrecto, mostrar un mensaje de error")
		// Mostrar un mensaje de error en la página de inicio de sesión
	}
}
