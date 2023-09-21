package main

import (
	"container/list"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

type Specifications struct {
	Name   string `json:"nombre"`
	Type   string `json:"tipo"`
	Memory int    `json:"memoria"`
	CPU    int    `json:"cpu"`
}

type Account struct {
	Username string `json:"nombre"`
	Password string `json:"contrasenia"`
}

type SpecificationsQueue struct {
	sync.Mutex
	Queue *list.List
}

type AccountsQueue struct {
	sync.Mutex
	Queue *list.List
}

var (
	specificationsQueue SpecificationsQueue
	accountsQueue       AccountsQueue
	mu                  sync.Mutex
	lastQueueSize       int
)

func main() {

	// Conexión a SQL
	manageSqlConecction()

	// Configura un manejador de solicitud para la ruta "/json".
	manageServer()

	// Función que verifica la cola de especificaciones constantemente.
	go checkSpecificationsQueueChanges()

	// Función que verifica la cola de cuentas constantemente.
	go checkAccountsQueueChanges()

	// Inicia el servidor HTTP en el puerto 8081.
	fmt.Println("Servidor escuchando en el puerto 8081...")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		fmt.Println("Error al iniciar el servidor:", err)
	}
}

func manageSqlConecction() {
	var err error
	db, err = sql.Open("mysql", "root:root@tcp(172.17.0.2:3306)/decktop_cloud_data_base")
	if err != nil {
		log.Fatal(err)
	}
}

func manageServer() {
	specificationsQueue.Queue = list.New()
	accountsQueue.Queue = list.New()

	http.HandleFunc("/json/specifications", func(w http.ResponseWriter, r *http.Request) {
		// Verifica que la solicitud sea del método POST.
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		// Decodifica el JSON recibido en la solicitud en una estructura Specifications.
		var specifications Specifications
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&specifications); err != nil {
			http.Error(w, "Error al decodificar JSON de especificaciones", http.StatusBadRequest)
			return
		}

		// Encola las especificaciones.
		mu.Lock()
		specificationsQueue.Queue.PushBack(specifications)
		mu.Unlock()

		fmt.Println("Cantidad de Solicitudes de Especificaciones en la Cola: " + strconv.Itoa(specificationsQueue.Queue.Len()))

		// Envía una respuesta al cliente.
		response := map[string]string{"mensaje": "Mensaje JSON de especificaciones recibido correctamente"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/json/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var account Account
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&account); err != nil {
			http.Error(w, "Error al decodificar JSON de inicio de sesión", http.StatusBadRequest)
			return
		}

		printAccount(account)

		// Si las credenciales son válidas, devuelve un JSON con "loginCorrecto" en true, de lo contrario, en false.
		query := "SELECT username FROM account WHERE username = ? AND password = ?"
		var resultUsername string

		err := db.QueryRow(query, account.Username, account.Password).Scan(&resultUsername)
		if err == sql.ErrNoRows {
			fmt.Println("Usuario no encontrado.")
			response := map[string]bool{"loginCorrecto": false}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(response)
		} else if err != nil {
			panic(err.Error())
		} else {
			fmt.Printf("Usuario encontrado: %s\n", resultUsername)
			response := map[string]bool{"loginCorrecto": true}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}
	})

}

func checkSpecificationsQueueChanges() {
	for {
		// Verifica si el tamaño de la cola de especificaciones ha cambiado.
		mu.Lock()
		currentQueueSize := specificationsQueue.Queue.Len()
		mu.Unlock()

		if currentQueueSize > 0 {
			// Imprime y elimina el primer elemento de la cola de especificaciones.
			mu.Lock()
			firstElement := specificationsQueue.Queue.Front()
			specificationsQueue.Queue.Remove(firstElement)
			mu.Unlock()

			// Procesa el primer elemento (en este caso, imprime las especificaciones).
			fmt.Println("Especificaciones eliminadas de la cola:")
			printSpecifications(firstElement.Value.(Specifications))
		}

		// Espera un segundo antes de verificar nuevamente.
		time.Sleep(1 * time.Second)
	}
}

func checkAccountsQueueChanges() {
	for {
		// Verifica si el tamaño de la cola de cuentas ha cambiado.
		mu.Lock()
		currentQueueSize := accountsQueue.Queue.Len()
		mu.Unlock()

		if currentQueueSize > 0 {
			// Imprime y elimina el primer elemento de la cola de cuentas.
			mu.Lock()
			firstElement := accountsQueue.Queue.Front()
			accountsQueue.Queue.Remove(firstElement)
			mu.Unlock()

			// Procesa el primer elemento (en este caso, imprime las cuentas).
			fmt.Println("Cuentas eliminadas de la cola:")
			printAccount(firstElement.Value.(Account))
		}

		// Espera un segundo antes de verificar nuevamente.
		time.Sleep(1 * time.Second)
	}
}

func printSpecifications(specs Specifications) {
	// Crea el comando en VirtualBox
	comandCreate := "Vboxmanage createvm --name " + specs.Name + " --ostype " + specs.Type
	comandModify := "Vboxmanage modifyvm " + specs.Name + " --memory " + strconv.Itoa(specs.Memory) + " --vram 128"

	// Imprime las especificaciones recibidas.
	fmt.Printf("-------------------------\n")
	fmt.Printf("Nombre de la Máquina: %s\n", specs.Name)
	fmt.Printf("Sistema Operativo: %s\n", specs.Type)
	fmt.Printf("Memoria Requerida: %d Mb\n", specs.Memory)
	fmt.Printf("CPU Requerida: %d núcleos\n", specs.CPU)
	fmt.Printf("Comando Crear en VirtualBox: %s\n", comandCreate)
	fmt.Printf("Comando Modificar en VirtualBox: %s\n", comandModify)
}

func printAccount(account Account) {
	// Imprime la cuenta recibida.
	fmt.Printf("-------------------------\n")
	fmt.Printf("Nombre de Usuario: %s\n", account.Username)
	fmt.Printf("Contraseña: %s\n", account.Password)
}
