package main

import (
	"container/list"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// Variable que almacena la ruta de la llave privada ingresada por paametro cuando de ejecuta el programa
var privateKeyPath = flag.String("key", "", "Ruta de la llave privada SSH")

// Cola de especificaciones para la gestiòn de màquinas virtuales
// La gestiòn puede ser: modificar, eliminar, iniciar, detener una MV.
type ManagementQueue struct {
	sync.Mutex
	Queue *list.List
}

/*
Estrucutura de datos tipo JSON que contiene los campos necesarios para la gestiòn de usuarios
@Nombre Representa el nombre del usuario
@Apellido Representa el apellido del usuario
@Email Representa el email del usuario
@Contrasenia Representa la contraseña de la cuenta
@Rol Representa el rol que tiene la persona en la plataforma. Puede ser Estudiante o Administrador
*/
type Persona struct {
	Nombre      string
	Apellido    string
	Email       string
	Contrasenia string
	Rol         string
}

/*
Estructura de datos tipo JSOn que contiene los datos de una màquina virtual
@Uuid Representa el uuid de una màqina virtual, el cual es un identificador ùnico
@Nombre Representa el nombre de la MV
@Ram Representa la cantidad de memoria RAM que tiene la màquina virtual
@Cpu Representa la cantidad de unidades de procesamiento que tiene la màquina virtial
@Ip Representa la direcciòn IP de la màquina
@Estado Representa el estado actual de la MV. Puede ser: Encendido, Apagado ò Procesando. Este ùltimo estado indica que la màquina se està encendiendo o apagando
@Hostname Representa el nombre del usuario del sistema operativo
@Persona_email Representa el email de la persona asociada a la MV.
@Host_id Representa el identificador ùnico de la màquina host en la cual està creada la MV
@Disco_id Representa el identificador ùnico del disco al cual està conectada la MV
@Sistema_operativo Represneta el tipo de sistema operativo que tiene la MV. Por ejemplo: Linux o Windows
@Distribucion_sistema_operativo Representa la distribuciòn del sistema operativo que està usando la MV. Por ejemplo: Debian ò 11 Home
*/
type Maquina_virtual struct {
	Uuid                           string
	Nombre                         string
	Ram                            int
	Cpu                            int
	Ip                             string
	Estado                         string
	Hostname                       string
	Persona_email                  string
	Host_id                        int
	Disco_id                       int
	Sistema_operativo              string
	Distribucion_sistema_operativo string
	Fecha_creacion                 time.Time
}

type Maquina_virtualQueue struct {
	sync.Mutex
	Queue *list.List
}

/*
Estructura de datos tipo JSON que contiene los campos de un host
@Id Representa el identificador ùnico del host
@Nombre Representa el nombre del host
@Mac Representa la direcciòn fìsica del host
@Ip Representa la direcciòn Ip del host
@Hostname Representa el nombre del host
@Ram_total Representa la cantidad total de memoria RAM que tiene el host. Se representa en mb
@Cpu_total Representa la cantidad de unidades de procesamiento total que tiene el host
@Almacentamiento_total Representa la cantidad total de almacenamiento del host. Se representa en mb
@Ram_usada Representa la cantidad total de memoria RAM que està siendo usada por las màquinas virtuales alojadas en el host. Se representa en mb
@Cpu_usada Representa la cantidad total de unidades de procesamiento que estàn siendo usadas por las MV's alojadas en el host
@Almacenamiento_usado Representa la cantidad de alamacenamiento que està siendo usado por las MV's alojadas en el host. Se representa en mb
@Adaptador_red Representa el nombre del adaptador de red del host
@Estado Representa el estado del host (Disponible o Fuera de servicio)
@Ruta_llave_ssh_pub Representa la ubiaciòn de la llave ssh pùblica
@Sistema_operativo Representa el tipo de sistema operativo del host. Por ejemplo: Windows o Mac
@Distribucion_sistema_operativo Representa el tipo de distribuciòn del sistema operativo que tiene el host. Por ejemplo: 10 Pro o 11 Home
*/
type Host struct {
	Id                             int
	Nombre                         string
	Mac                            string
	Ip                             string
	Hostname                       string
	Ram_total                      int
	Cpu_total                      int
	Almacenamiento_total           int
	Ram_usada                      int
	Cpu_usada                      int
	Almacenamiento_usado           int
	Adaptador_red                  string
	Estado                         string
	Ruta_llave_ssh_pub             string
	Sistema_operativo              string
	Distribucion_sistema_operativo string
}

/*
Estructura de datos tipo JSON que contiene los campos para representar una MV del catàlogo
@Nombre Representa el nombre de la MV
@Memoria Representa la cantidad de memoria RAM de la MV
@Cpu Representa la cantidad de unidades de procesamiento de la MV
@Sistema_operativo Representa el tipo de sistema operativo de la Mv
@Distribucion_sistema_operativo Representa la distribuciòn del sistema operativo que tiene la màquina del catàlogo
@Arquitectura Respresenta la arquitectura del sistema operativo. Se presententa en un valor entero. Por ejemplo: 32 o 64
*/
type Catalogo struct {
	Id                             int
	Nombre                         string
	Ram                            int
	Cpu                            int
	Sistema_operativo              string
	Distribucion_sistema_operativo string
	Arquitectura                   int
}

/*
Estructura de datos tipo JSON que representa la informaciòn de los discos que tiene la plataforma Desktop Cloud
@Id Representa el identificador ùnico del disco en la base de datos. Este identificador es generado automaticamente por la base de datos
@Nombre Representa el nombre del disco
@Ruta_ubicacion Representa la ubicaciòn de disco en el host.
@Sistema_operativo Representa el tipo de sistema operativo que tiene el disco. Por ejemplo: Linux
@Distribucion_sistema_operativo Representa el tipo de distribuciòn del sistema operativo. Por ejemplo: Debian o Ubuntu
@arquitectura Representa la arquitectura del sistema operativo. Se representa en un valor entero. Por ejemplo: 32 o 64
@Host_id Representa el identificador ùnico del host en el cual està ubicado el disco
*/
type Disco struct {
	Id                             int
	Nombre                         string
	Ruta_ubicacion                 string
	Sistema_operativo              string
	Distribucion_sistema_operativo string
	arquitectura                   int
	Host_id                        int
}

// Declaraciòn de variables globales
var (
	maquina_virtualesQueue Maquina_virtualQueue
	managementQueue        ManagementQueue
	mu                     sync.Mutex
	lastQueueSize          int
)

func main() {

	flag.Parse()

	//Verifica que el paràmetro de la ruta de la llave privada no estè vacìo
	if *privateKeyPath == "" {
		fmt.Println("Debe ingresar la ruta de la llave privada SSH")
		return
	}

	// Conexión a SQL
	manageSqlConecction()

	// Configura un manejador de solicitud para la ruta "/json".
	manageServer()

	// Función que verifica la cola de especificaciones constantemente.
	go checkMaquinasVirtualesQueueChanges()

	//Funciòn que verifica el tiempo de creaciòn de una MV
	//go checkTime()

	// Función que verifica la cola de cuentas constantemente.
	go checkManagementQueueChanges()

	// Inicia el servidor HTTP en el puerto 8081.
	fmt.Println("Servidor escuchando en el puerto 8081...")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Println("Error al iniciar el servidor:", err)
	}

}

/*
Funciòn que establece un disparador cada 10 minutos el cual invoca la funciòn checkMachineTime
*/

func checkTime() {

	timeTicker := time.NewTicker(10 * time.Minute) // Se ejecuta cada diez minutos

	for {
		select {
		case <-timeTicker.C:
			go checkMachineTime()
		}
	}
}

// Funciòn que se encarga de realizar la conexiòn a la base de datos

func manageSqlConecction() {
	var err error

	db, err = sql.Open("mysql", "root:root@tcp(172.17.0.2)/uqcloud")
	if err != nil {
		log.Fatal(err)
	}

}

/*
Funciòn que se encarga de configurar los endpoints, realizar las validaciones correspondientes a los JSON que llegan
por solicitudes HTTP. Se encarga tambièn de ingresar las peticiones para gestiòn de MV a la cola.
Si la peticiòn es de inicio de sesiòn, la gestiona inmediatamente.
*/
func manageServer() {
	maquina_virtualesQueue.Queue = list.New()
	managementQueue.Queue = list.New()

	//Endpoint para las peticiones de creaciòn de màquinas virtuales
	http.HandleFunc("/json/createVirtualMachine", func(w http.ResponseWriter, r *http.Request) {
		// Verifica que la solicitud sea del método POST.
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		// Decodifica el JSON recibido en la solicitud en una estructura Specifications.
		var payload map[string]interface{}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "Error al decodificar JSON de la solicitud", http.StatusBadRequest)
			return
		}

		// Encola las especificaciones.
		mu.Lock()
		maquina_virtualesQueue.Queue.PushBack(payload)
		mu.Unlock()

		fmt.Println("Cantidad de Solicitudes de Especificaciones en la Cola: " + strconv.Itoa(maquina_virtualesQueue.Queue.Len()))

		// Envía una respuesta al cliente.
		response := map[string]string{"mensaje": "Mensaje JSON de crear MV recibido correctamente"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	//Endpoint para peticiones de inicio de sesiòn
	http.HandleFunc("/json/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}
		var persona Persona
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&persona); err != nil {
			http.Error(w, "Error al decodificar JSON de inicio de sesión", http.StatusBadRequest)
			return
		}

		// Si las credenciales son válidas, devuelve un JSON con "loginCorrecto" en true, de lo contrario, en false.
		query := "SELECT contrasenia FROM persona WHERE email = ?"
		var resultPassword string

		//Consulta en la base de datos si el usuario existe
		err := db.QueryRow(query, persona.Email).Scan(&resultPassword)

		err2 := bcrypt.CompareHashAndPassword([]byte(resultPassword), []byte(persona.Contrasenia))
		if err2 != nil {
			fmt.Println("Contraseña incorrecta")
		} else {
			fmt.Println("Contraseña correcta")
		}
		if err2 != nil {
			response := map[string]interface{}{
				"loginCorrecto": false,
				"usuario":       nil,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(response)
		} else if err != nil {
			panic(err.Error())
		} else {
			// Consulta en la base de datos para obtener los detalles del usuario
			queryUsuario := "SELECT * FROM persona WHERE email = ?"
			var usuario Persona

			errUsuario := db.QueryRow(queryUsuario, persona.Email).Scan(&usuario.Email, &usuario.Nombre, &usuario.Apellido, &usuario.Contrasenia, &usuario.Rol)
			if errUsuario != nil {
				fmt.Println("Error al obtener detalles del usuario:", errUsuario)
				response := map[string]interface{}{
					"loginCorrecto": false,
					"usuario":       nil,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(response)
				return
			}

			response := map[string]interface{}{
				"loginCorrecto": true,
				"usuario":       usuario,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}
	})

	//Endpoint para peticiones de inicio de sesiòn
	http.HandleFunc("/json/signin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var persona Persona
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&persona); err != nil {
			http.Error(w, "Error al decodificar JSON de inicio de sesión", http.StatusBadRequest)
			return
		}

		printAccount(persona)

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(persona.Contrasenia), bcrypt.DefaultCost)
		if err != nil {
			fmt.Println("Error al encriptar la contraseña:", err)
			return
		}

		query := "INSERT INTO persona (nombre, apellido, email, contrasenia, rol) VALUES ( ?, ?, ?, ?, ?);"
		var resultUsername string

		//Registra el usuario en la base de datos
		_, err = db.Exec(query, persona.Nombre, persona.Apellido, persona.Email, hashedPassword, "Estudiante")
		if err != nil {
			fmt.Println("Error al registrar.")
			response := map[string]bool{"loginCorrecto": false}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(response)
		} else if err != nil {
			panic(err.Error())
		} else {
			fmt.Printf("Registro correcto: %s\n", resultUsername)
			response := map[string]bool{"loginCorrecto": true}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}
	})

	//Endpoint para consultar las màquinas virtuales de un usuario
	http.HandleFunc("/json/consultMachine", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var persona Persona
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&persona); err != nil { //Solo llega el email
			http.Error(w, "Error al decodificar JSON de inicio de sesión", http.StatusBadRequest)
			return
		}

		persona, error := getUser(persona.Email)
		if error != nil {
			return
		}

		machines, err := consultMachines(persona)
		if err != nil && err.Error() != "no Machines Found" {
			fmt.Println(err)
			log.Println("Error al consultar las màquinas del usuario")
			return
		}

		// Respondemos con la lista de máquinas virtuales en formato JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(machines)

	})

	//Endpoint para consultar los Host
	http.HandleFunc("/json/consultHost", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var persona Persona
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&persona); err != nil { //Solo llega el email
			http.Error(w, "Error al decodificar JSON de inicio de sesión", http.StatusBadRequest)
			return
		}

		persona, error := getUser(persona.Email)
		if error != nil {
			return
		}

		hosts, err := consultHosts()
		if err != nil && err.Error() != "no Hosts encontrados" {
			fmt.Println(err)
			log.Println("Error al consultar los Host")
			return
		}

		// Respondemos con la lista de máquinas virtuales en formato JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(hosts)

	})

	//Endpoint para consultar el catàlogo
	http.HandleFunc("/json/consultCatalog", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Se requiere una solicitud Get", http.StatusMethodNotAllowed)
			return
		}

		catalogo, err := consultCatalog()
		if err != nil {
			log.Printf("Error al consultar el catálogo: %v", err)
			http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
			return
		}

		// Respondemos con la lista de máquinas virtuales en formato JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(catalogo); err != nil {
			log.Printf("Error al codificar la respuesta JSON: %v", err)
			http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
			return
		}

	})

	//End point para modificar màquinas virtuales
	http.HandleFunc("/json/modifyVM", func(w http.ResponseWriter, r *http.Request) {
		// Verifica que la solicitud sea del método POST.
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		// Decodifica el JSON recibido en la solicitud en un mapa genérico.
		var payload map[string]interface{}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "Error al decodificar JSON de la solicitud", http.StatusBadRequest)
			return
		}

		// Verifica que el campo "tipo_solicitud" esté presente y sea "modify".
		tipoSolicitud, isPresent := payload["tipo_solicitud"].(string)
		if !isPresent || tipoSolicitud != "modify" {
			http.Error(w, "El campo 'tipo_solicitud' debe ser 'modify'", http.StatusBadRequest)
			return
		}

		// Extrae el objeto "specifications" del JSON.
		specificationsData, isPresent := payload["specifications"].(map[string]interface{})
		if !isPresent || specificationsData == nil {
			http.Error(w, "El campo 'specifications' es inválido", http.StatusBadRequest)
			return
		}

		// Encola las peticiones.
		mu.Lock()
		managementQueue.Queue.PushBack(payload)
		mu.Unlock()

		// Envía una respuesta al cliente.
		response := map[string]string{"mensaje": "Mensaje JSON de especificaciones para modificar MV recibido correctamente"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	//End point para eliminar màquinas virtuales
	http.HandleFunc("/json/deleteVM", func(w http.ResponseWriter, r *http.Request) {
		// Verifica que la solicitud sea del método POST.
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var datos map[string]interface{}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&datos); err != nil {
			http.Error(w, "Error al decodificar JSON de especificaciones", http.StatusBadRequest)
			return
		}

		// Verificar si el nombre de la máquina virtual, la IP del host y el tipo de solicitud están presentes y no son nulos
		nombre, nombrePresente := datos["nombreVM"].(string)
		tipoSolicitud, tipoPresente := datos["tipo_solicitud"].(string)

		if !tipoPresente || tipoSolicitud != "delete" {
			http.Error(w, "El campo 'tipo_solicitud' debe ser 'delete'", http.StatusBadRequest)
			return
		}

		if !nombrePresente || !tipoPresente || nombre == "" || tipoSolicitud == "" {
			http.Error(w, "El tipo de solicitud y el nombre de la máquina virtual son obligatorios", http.StatusBadRequest)
			return
		}

		// Encola las peticiones.
		mu.Lock()
		managementQueue.Queue.PushBack(datos)
		mu.Unlock()

		// Envía una respuesta al cliente.
		response := map[string]string{"mensaje": "Mensaje JSON para eliminar MV recibido correctamente"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)

	})

	//End point para encender màquinas virtuales
	http.HandleFunc("/json/startVM", func(w http.ResponseWriter, r *http.Request) {
		// Verifica que la solicitud sea del método POST.
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var datos map[string]interface{}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&datos); err != nil {
			http.Error(w, "Error al decodificar JSON de especificaciones", http.StatusBadRequest)
			return
		}

		// Verificar si el nombre de la máquina virtual, la IP del host y el tipo de solicitud están presentes y no son nulos
		nombreVM, nombrePresente := datos["nombreVM"].(string)

		tipoSolicitud, tipoPresente := datos["tipo_solicitud"].(string)

		if !tipoPresente || tipoSolicitud != "start" {
			http.Error(w, "El campo 'tipo_solicitud' debe ser 'start'", http.StatusBadRequest)
			return
		}

		if !nombrePresente || !tipoPresente || nombreVM == "" || tipoSolicitud == "" {
			http.Error(w, "El tipo de solicitud y nombre de la máquina virtual son obligatorios", http.StatusBadRequest)
			return
		}

		// Encola las peticiones.
		mu.Lock()
		managementQueue.Queue.PushBack(datos)
		mu.Unlock()

		query := "select estado from maquina_virtual where nombre = ?"
		var estado string

		//Registra el usuario en la base de datos
		db.QueryRow(query, nombreVM).Scan(&estado)

		mensaje := "Apagando "
		if estado == "Apagado" {
			mensaje = "Encendiendo "
		}

		// Envía una respuesta al cliente.
		response := map[string]string{"mensaje": mensaje}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)

	})

	//End point para apagar màquinas virtuales
	http.HandleFunc("/json/stopVM", func(w http.ResponseWriter, r *http.Request) {
		// Verifica que la solicitud sea del método POST.
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var datos map[string]interface{}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&datos); err != nil {
			http.Error(w, "Error al decodificar JSON de especificaciones", http.StatusBadRequest)
			return
		}

		// Verificar si el nombre de la máquina virtual, la IP del host y el tipo de solicitud están presentes y no son nulos
		nombreVM, nombrePresente := datos["nombreVM"].(string)

		tipoSolicitud, tipoPresente := datos["tipo_solicitud"].(string)

		if !tipoPresente || tipoSolicitud != "stop" {
			http.Error(w, "El campo 'tipo_solicitud' debe ser 'stop'", http.StatusBadRequest)
			return
		}

		if !nombrePresente || !tipoPresente || nombreVM == "" || tipoSolicitud == "" {
			http.Error(w, "El tipo de solicitud y nombre de la máquina virtual son obligatorios", http.StatusBadRequest)
			return
		}

		// Encola las peticiones.
		mu.Lock()
		managementQueue.Queue.PushBack(datos)
		mu.Unlock()

		// Envía una respuesta al cliente.
		response := map[string]string{"mensaje": "Mensaje JSON para apagar MV recibido correctamente"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)

	})

	http.HandleFunc("/json/createGuestMachine", func(w http.ResponseWriter, r *http.Request) {
		// Verifica que la solicitud sea del método POST.
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var datos map[string]interface{}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&datos); err != nil {
			http.Error(w, "Error al decodificar el JSON ", http.StatusBadRequest)
			return
		}

		clientIP := datos["ip"].(string)
		distribucion_SO := datos["distribucion"].(string)
		email := createTempAccount(clientIP, distribucion_SO)

		// Envía una respuesta al cliente.
		response := map[string]string{"mensaje": email}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)

	})

	http.HandleFunc("/json/addHost", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var host Host

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&host); err != nil {
			http.Error(w, "Error al decodificar JSON de especificaciones", http.StatusBadRequest)
			return
		}

		query := "insert into host (nombre, mac, ip, hostname, ram_total, cpu_total, almacenamiento_total, adaptador_red, estado, ruta_llave_ssh_pub, sistema_operativo, distribucion_sistema_operativo) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

		//Registra el usuario en la base de datos
		_, err := db.Exec(query, host.Nombre, host.Mac, host.Ip, host.Hostname, host.Ram_total, host.Cpu_total, host.Almacenamiento_total, host.Adaptador_red, "Activo", host.Ruta_llave_ssh_pub, host.Sistema_operativo, host.Distribucion_sistema_operativo)
		if err != nil {
			fmt.Println("Error al registrar el host.")

		} else if err != nil {
			panic(err.Error())
		}

		fmt.Println("Registro del host exitoso")
		response := map[string]bool{"registroCorrecto": true}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/json/addDisk", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Se requiere una solicitud POST", http.StatusMethodNotAllowed)
			return
		}

		var disco Disco

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&disco); err != nil {
			http.Error(w, "Error al decodificar JSON de especificaciones", http.StatusBadRequest)
			return
		}

		query := "insert into disco (nombre, ruta_ubicacion, sistema_operativo, distribucion_sistema_operativo, arquitectura, host_id) values (?, ?, ?, ?, ?, ?);"

		_, err := db.Exec(query, disco.Nombre, disco.Ruta_ubicacion, disco.Sistema_operativo, disco.Distribucion_sistema_operativo, disco.arquitectura, disco.Host_id)
		if err != nil {
			log.Println("Error al registrar el disco.")
			return

		} else if err != nil {
			panic(err.Error())
		}
		fmt.Println("Registro del disco exitoso")
		response := map[string]bool{"registroCorrecto": true}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/json/consultMetrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Se requiere una solicitud GET", http.StatusMethodNotAllowed)
			return
		}

		metricas, err := getMetrics()

		if err != nil {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(metricas)
	})

}

func checkMaquinasVirtualesQueueChanges() {
	for {
		// Verifica si el tamaño de la cola de especificaciones ha cambiado.
		mu.Lock()
		currentQueueSize := maquina_virtualesQueue.Queue.Len()
		mu.Unlock()

		if currentQueueSize > 0 {
			// Imprime y elimina el primer elemento de la cola de especificaciones.
			mu.Lock()
			firstElement := maquina_virtualesQueue.Queue.Front()
			data, dataPresent := firstElement.Value.(map[string]interface{})
			//maquina_virtualesQueue.Queue.Remove(firstElement)
			mu.Unlock()

			if !dataPresent {
				fmt.Println("No se pudo procesar la solicitud")
				mu.Lock()
				maquina_virtualesQueue.Queue.Remove(firstElement)
				mu.Unlock()
				continue
			}

			specsMap, _ := data["specifications"].(map[string]interface{})
			specsJSON, err := json.Marshal(specsMap)
			if err != nil {
				fmt.Println("Error al serializar las especificaciones:", err)
				mu.Lock()
				maquina_virtualesQueue.Queue.Remove(firstElement)
				mu.Unlock()
				continue
			}

			var specifications Maquina_virtual
			err = json.Unmarshal(specsJSON, &specifications)
			if err != nil {
				fmt.Println("Error al deserializar las especificaciones:", err)
				mu.Lock()
				maquina_virtualesQueue.Queue.Remove(firstElement)
				mu.Unlock()
				continue
			}

			clientIP := data["clientIP"].(string)

			go crateVM(specifications, clientIP)
			maquina_virtualesQueue.Queue.Remove(firstElement)
			printMaquinaVirtual(specifications, true)
		}

		// Espera un segundo antes de verificar nuevamente.
		time.Sleep(1 * time.Second)
	}
}

func printMaquinaVirtual(specs Maquina_virtual, isCreateVM bool) {

	// Imprime las especificaciones recibidas.
	fmt.Printf("-------------------------\n")
	fmt.Printf("Nombre de la Máquina: %s\n", specs.Nombre)
	fmt.Printf("Sistema Operativo: %s\n", specs.Sistema_operativo)
	fmt.Printf("Distribuciòn SO: %s\n", specs.Distribucion_sistema_operativo)
	fmt.Printf("Memoria Requerida: %d Mb\n", specs.Ram)
	fmt.Printf("CPU Requerida: %d núcleos\n", specs.Cpu)

}

func printAccount(account Persona) {
	// Imprime la cuenta recibida.
	fmt.Printf("-------------------------\n")
	fmt.Printf("Nombre de Usuario: %s\n", account.Nombre)
	fmt.Printf("Contraseña: %s\n", account.Contrasenia)
	fmt.Printf("Email: %s\n", account.Email)

}

/*
Esta funciòn carga y devuelve la llave privada SSH desde la ruta especificada
@file Paràmetro que contiene la ruta de la llave privada
*/
func privateKeyFile(file string) (ssh.AuthMethod, error) {
	buffer, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, err
	}

	return ssh.PublicKeys(key), nil
}

/*
Funciòn que se encarga de realizar la configuraciòn SSH con el host
@user Paràmetro que contiene el nombre del usuario al cual se va a conectar
@privateKeyPath Paràmetro que contiene la ruta de la llave privada SSH
*/
func configurarSSH(user string, privateKeyPath string) (*ssh.ClientConfig, error) {
	authMethod, err := privateKeyFile(privateKeyPath)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	return config, nil
}

/*
	Funciòn que se encarga de enviar los comandos a travès de la conexiòn SSH con el host

@host Paràmetro que contien la direcciòn IP del host al cual le va a enviar los comandos
@comando Paràmetro que contiene la instrucciòn que se desea ejecutar en el host
@config Paràmetro que contiene la configuraciòn SSH
@return Retorna la respuesta del host si la hay
*/
func enviarComandoSSH(host string, comando string, config *ssh.ClientConfig) (salida string, err error) {

	//Establece la conexiòn SSH
	conn, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		log.Println("Error al establecer la conexiòn SSH: ", err)
		return "", err
	}
	defer conn.Close()

	//Crea una nueva sesiòn SSH
	session, err := conn.NewSession()
	if err != nil {
		log.Println("Error al crear la sesiòn SSH: ", err)
		return "", err
	}
	defer session.Close()
	//Ejecuta el comando remoto
	output, err := session.CombinedOutput(comando)
	if err != nil {
		log.Println("Error al ejecutar el comando remoto: " + string(output))
		return "", err
	}
	return string(output), nil
}

/*
	Esta funciòn permite enviar los comandos VBoxManage necesarios para crear una nueva màquina virtual
	Se encarga de verificar si el usuario aùn puede crear màquinas virtuales, dependiendo de su rol: màximo 5 para estudiantes y màximo 3 para invitados
	Se encarga de escoger el host dependiendo desde donde se hace la solicitud: algoritmo "here" si se realiza desde un computador que pertenece a los host ò aleatorio en caso contrario
	Valida si el host que se escogiò tiene recursos disponibles para crear la MV solicitada
	Finalmente, crea y actualiza los registros necesarios en la base de datos

@spects Paràmetro que contiene la configuraciòn enviada por el usuario para crear la MV
@clientIP Paràmetro que contiene la direcciòn IP de la màquina desde la cual se està realizando la peticiòn para crear la MV
*/
func crateVM(specs Maquina_virtual, clientIP string) string {

	//Obtiene el usuario
	user, error0 := getUser(specs.Persona_email)
	if error0 != nil {
		log.Println("Error al obtener el usuario")
		return ""
	}

	//Valida el nùmero de màquinas virtuales que tiene el usuario
	/*cantidad, err0 := countUserMachinesCreated(specs.Persona_email)
	if err0 != nil {
		log.Println("Error al obtener la cantidad de màquinas que tiene el usuario")
		return ""
	}*/

	if user.Rol == "Estudiante" {
		/*if cantidad >= 5 {
			fmt.Println("El usuario " + user.Nombre + " no puede crear màs de 5 màquinas virtuales.")
			return "El usuario " + user.Nombre + " no puede crear màs de 5 màquinas virtuales."
		}*/
	}

	if user.Rol == "Invitado" {
		/*if cantidad >= 3 {
			fmt.Println("El usuario invitado no puede crear màs de 1 màquina virtual.")
			return "El usuario invitado no puede crear màs de 1 màquina virtual."
		}*/
	}

	caracteres := generateRandomString(4) //Genera 4 caracteres alfanumèricos para concatenarlos al nombre de la MV

	nameVM := specs.Nombre + "_" + caracteres

	//Consulta si existe una MV con ese nombre
	existe, error1 := existVM(nameVM)
	if error1 != nil {
		if error1 != sql.ErrNoRows {
			log.Println("Error al consultar si existe una MV con el nombre indicado: ", error1)
			return "Error al consultar si existe una MV con el nombre indicado"
		}
	} else if existe {
		fmt.Println("El nombre " + nameVM + " no està disponible, por favor ingrese otro.")
		return "Nombre de la MV no disponible"
	}

	var host Host
	availableResources := false
	host, er := isAHostIp(clientIP) //Consulta si la ip de la peticiòn proviene de un host registrado en la BD
	if er == nil {                  //nil = El host existe
		availableResources = validarDisponibilidadRecursosHost(specs.Cpu, specs.Ram, host) //Verifica si el host tiene recursos disponibles
	}

	//Obtiene la cantidad total de hosts que hay en la base de datos
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM host").Scan(&count)
	if err != nil {
		log.Println("Error al contar los host que hay en la base de datos: " + err.Error())
		return "Error al contar los gost que hay en la base de datos"
	}

	count += 5 //Para dar n+5 iteraciones en busca de hosts con recursos disponibles, donde n es el total de hosts guardados en la bse de datos

	//Escoge hosts al azar en busca de alguno que tenga recursos disponibles para crear la MV
	for !availableResources && count > 0 {
		//Selecciona un host al azar
		host, err = selectHost()
		if err != nil {
			log.Println("Error al seleccionar el host:", err)
			return "Error al seleccionar el host"
		}
		availableResources = validarDisponibilidadRecursosHost(specs.Cpu, specs.Ram, host) //Verifica si el host tiene recursos disponibles
		count--
	}

	if !availableResources {
		fmt.Println("No hay recursos disponibles el Desktop Cloud para crear la màquina virtual. Intente màs tarde")
		return "No hay recursos disponibles el Desktop Cloud para crear la màquina virtual. Intente màs tarde"
	}

	//Obtiene el disco que cumpla con los requerimientos del cliente
	disco, err20 := getDisk(specs.Sistema_operativo, specs.Distribucion_sistema_operativo, host.Id)
	if err20 != nil {
		log.Println("Error al obtener el disco:", err20)
		return "Error al obtener el disco"
	}

	//Configura la conexiòn SSH con el host
	config, err := configurarSSH(host.Hostname, *privateKeyPath)
	if err != nil {
		log.Println("Error al configurar SSH:", err)
		return "Error al configurar la conexiòn SSH"
	}

	//Comando para crear una màquina virtual
	createVM := "VBoxManage createvm --name " + "\"" + nameVM + "\"" + " --ostype " + disco.Distribucion_sistema_operativo + "_" + strconv.Itoa(disco.arquitectura) + " --register"
	uuid, err1 := enviarComandoSSH(host.Ip, createVM, config)
	if err1 != nil {
		log.Println("Error al ejecutar el comando para crear y registrar la MV:", err1)
		return "Error al crear la MV"
	}

	//Comando para asignar la memoria RAM a la MV
	memoryCommand := "VBoxManage modifyvm " + "\"" + nameVM + "\"" + " --memory " + strconv.Itoa(specs.Ram)
	_, err2 := enviarComandoSSH(host.Ip, memoryCommand, config)
	if err2 != nil {
		log.Println("Error ejecutar el comando para asignar la memoria a la MV:", err2)
		return "Error al asignar la memoria a la MV"
	}

	//Comando para agregar el controlador de almacenamiento
	sctlCommand := "VBoxManage storagectl " + "\"" + nameVM + "\"" + " --name hardisk --add sata"
	_, err3 := enviarComandoSSH(host.Ip, sctlCommand, config)
	if err3 != nil {
		log.Println("Error al ejecutar el comando para asignar el controlador de almacenamiento a la MV:", err3)
		return "Error al asignar el controlador de almacenamiento a la MV"
	}

	//Comando para conectar el disco multiconexiòn a la MV
	sattachCommand := "VBoxManage storageattach " + "\"" + nameVM + "\"" + " --storagectl hardisk --port 0 --device 0 --type hdd --medium " + "\"" + disco.Ruta_ubicacion + "\""
	_, err4 := enviarComandoSSH(host.Ip, sattachCommand, config)
	if err4 != nil {
		log.Println("Error al ejecutar el comando para conectar el disco a la MV: ", err4)
		return "Error al conectar el disco a la MV"
	}

	//Comando para asignar las unidades de procesamiento
	cpuCommand := "VBoxManage modifyvm " + "\"" + nameVM + "\"" + " --cpus " + strconv.Itoa(specs.Cpu)
	_, err5 := enviarComandoSSH(host.Ip, cpuCommand, config)
	if err5 != nil {
		log.Println("Error al ejecutar el comando para asignar la cpu a la MV:", err5)
		return "Error al asignar la cpu a la MV"
	}

	//Comando para poner el adaptador de red en modo puente (Bridge)
	redAdapterCommand := "VBoxManage modifyvm " + "\"" + nameVM + "\"" + " --nic1 bridged --bridgeadapter1 " + "\"" + host.Adaptador_red + "\""
	_, err6 := enviarComandoSSH(host.Ip, redAdapterCommand, config)
	if err6 != nil {
		log.Println("Error al ejecutar el comando para configurar el adaptador de red de la MV:", err6)
		return "Error al configurar el adaptador de red de la MV"
	}

	//Obtiene el UUID de la màquina virtual creda
	lines := strings.Split(string(uuid), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "UUID:") {
			uuid = strings.TrimPrefix(line, "UUID:")
		}
	}
	currentTime := time.Now().UTC()

	nuevaMaquinaVirtual := Maquina_virtual{
		Uuid:              uuid,
		Nombre:            nameVM,
		Sistema_operativo: specs.Sistema_operativo,
		Ram:               specs.Ram,
		Cpu:               specs.Cpu,
		Estado:            "Apagado",
		Hostname:          "uqcloud",
		Persona_email:     specs.Persona_email,
		Fecha_creacion:    currentTime,
	}

	//Crea el registro de la nueva MV en la base de datos
	_, err7 := db.Exec("INSERT INTO maquina_virtual (uuid, nombre,  ram, cpu, ip, estado, hostname, persona_email, host_id, disco_id, fecha_creacion) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		nuevaMaquinaVirtual.Uuid, nuevaMaquinaVirtual.Nombre, nuevaMaquinaVirtual.Ram, nuevaMaquinaVirtual.Cpu,
		nuevaMaquinaVirtual.Ip, nuevaMaquinaVirtual.Estado, nuevaMaquinaVirtual.Hostname, nuevaMaquinaVirtual.Persona_email,
		host.Id, disco.Id, nuevaMaquinaVirtual.Fecha_creacion)
	if err7 != nil {
		log.Println("Error al crear el registro en la base de datos:", err7)
		return "Error al crear el registro en la base de datos"
	}

	//Calcula la CPU y RAM usada en el host
	usedCpu := host.Cpu_usada + specs.Cpu
	usedRam := host.Ram_usada + (specs.Ram)

	//Actualiza la informaciòn de los recursos usados en el host
	_, err8 := db.Exec("UPDATE host SET ram_usada = ?, cpu_usada = ? where id = ?", usedRam, usedCpu, host.Id)
	if err8 != nil {
		log.Println("Error al actualizar el host en la base de datos: ", err8)
		return "Error al actualizar el host en la base de datos"
	}

	fmt.Println("Màquina virtual creada con èxito")
	startVM(nameVM, clientIP)
	return "Màquina virtual creada con èxito"
}

/*
	Esta funciòn verifica si una màquina virtual està encendida

@nameVM Paràmetro que contiene el nombre de la màquina virtual a verificar
@hostIP Paràmetro que contiene la direcciòn Ip del host en el cual està la MV
@return Retorna true si la màquina està encendida o false en caso contrario
*/
func isRunning(nameVM string, hostIP string, config *ssh.ClientConfig) (bool, error) {

	//Comando para saber el estado de una màquina virtual
	command := "VBoxManage showvminfo " + "\"" + nameVM + "\"" + " | findstr /C:\"State:\""
	running := false

	salida, err := enviarComandoSSH(hostIP, command, config)
	if err != nil {
		log.Println("Error al ejecutar el comando para obtener el estado de la màquina:", err)
		return running, err
	}

	// Expresión regular para buscar el estado (running)
	regex := regexp.MustCompile(`State:\s+(running|powered off)`)
	matches := regex.FindStringSubmatch(salida)

	// matches[1] contendrá "running" o "powered off" dependiendo del estado
	if len(matches) > 1 {
		estado := matches[1]
		if estado == "running" {
			running = true
		}
	}
	return running, nil
}

/* Funciòn que contiene los comandos necesarios para modificar una màquina virtual. Primero verifica
si la màquina esta encendida o apagada. En caso de que estè encendida, se le notifica el usuario que debe apagar la màquina.
Tambièn, verifica si el host tiene los recusos necesarios solicitados por el cliente, en caso de que requiera aumentar lso recursos.
@specs Paràmetro que contiene las especificaciones a modificar en la màquina virtual
*/

func modifyVM(specs Maquina_virtual) string {

	//Obtiene la màquina virtual a modificar
	maquinaVirtual, err1 := getVM(specs.Nombre)
	if err1 != nil {
		log.Println("Error al obtener la MV:", err1)
		return "Error al obtener la MV"
	}

	//Obtiene el host en el cual està alojada la MV
	host, err2 := getHost(maquinaVirtual.Host_id)
	if err2 != nil {
		log.Println("Error al obtener el host:", err2)
		return "Error al obtener el host"
	}

	//Configura la conexiòn SSH con el host
	config, err := configurarSSH(host.Hostname, *privateKeyPath)
	if err != nil {
		log.Println("Error al configurar SSH:", err)
		return "Error al configurar SSH"
	}

	//Comando para modificar la memoria RAM a la MV
	memoryCommand := "VBoxManage modifyvm " + "\"" + specs.Nombre + "\"" + " --memory " + strconv.Itoa(specs.Ram)

	//Comando para modificar las unidades de procesamiento
	cpuCommand := "VBoxManage modifyvm " + "\"" + specs.Nombre + "\"" + " --cpus " + strconv.Itoa(specs.Cpu)

	//Variable que contiene el estado de la MV (Encendida o apagada)
	running, err3 := isRunning(specs.Nombre, host.Ip, config)
	if err3 != nil {
		log.Println("Error al obtener el estado de la MV:", err3)
		return "Error al obtener el estado de la MV"
	}

	if running {
		fmt.Println("Para modificar la màquina primero debe apagarla")
		return "Para modificar la màquina primero debe apagarla"
	}

	if specs.Cpu != 0 && specs.Cpu != maquinaVirtual.Cpu {
		var cpu_host_usada int
		flagCpu := true

		if specs.Cpu < maquinaVirtual.Cpu {
			cpu_host_usada = host.Cpu_usada - (maquinaVirtual.Cpu - specs.Cpu)
		} else {
			cpu_host_usada = host.Cpu_usada + (specs.Cpu - maquinaVirtual.Cpu)
			recDisponibles := validarDisponibilidadRecursosHost((specs.Cpu - maquinaVirtual.Cpu), 0, host) //Valida si el host tiene recursos disponibles
			if !recDisponibles {
				fmt.Println("No se pudo aumentar la cpu, no hay recursos disponibles en el host")
				flagCpu = false
			}
		}
		if flagCpu {
			//ACtualiza la CPU usada en el host
			_, er := db.Exec("UPDATE host SET cpu_usada = ? where id = ?", cpu_host_usada, host.Id)
			if er != nil {
				log.Println("Error al actualizar la cpu_usada del host en la base de datos: ", er)
				return "Error al actualizar el host en la base de datos"
			}
			_, err11 := enviarComandoSSH(host.Ip, cpuCommand, config)
			if err11 != nil {
				log.Println("Error al realizar la actualizaciòn de la cpu", err11)
				return "Error al realizar la actualizaciòn de la cpu"
			}
			//Actualiza la CPU que tiene la MV
			_, err1 := db.Exec("UPDATE maquina_virtual set cpu = ? WHERE NOMBRE = ?", strconv.Itoa(specs.Cpu), specs.Nombre)
			if err1 != nil {
				log.Println("Error al realizar la actualizaciòn de la CPU", err1)
				return "Error al realizar la actualizaciòn de la CPU"
			}
			fmt.Println("Se modificò con èxito la CPU")
		}
	}

	if specs.Ram != 0 && specs.Ram != maquinaVirtual.Ram {
		var ram_host_usada int
		flagRam := true

		if specs.Ram < maquinaVirtual.Ram {
			ram_host_usada = host.Ram_usada - (maquinaVirtual.Ram - specs.Ram)
		} else {
			ram_host_usada = host.Ram_usada + (specs.Ram - maquinaVirtual.Ram)
			recDisponibles := validarDisponibilidadRecursosHost(0, (specs.Ram - maquinaVirtual.Ram), host) //Valida si el host tiene RAM disponible para realizar el aumento de recursos
			if !recDisponibles {
				fmt.Println("No se modificò la ram porque el host no tiene recursos disponibles")
				flagRam = false
			}
		}
		if flagRam {
			//Actualiza la RAM usada en el host
			_, er := db.Exec("UPDATE host SET ram_usada = ? where id = ?", ram_host_usada, host.Id)
			if er != nil {
				log.Println("Error al actualizar la ram_usada del host en la base de datos: ", er)
				return "Error al actualizar el host en la base de datos"
			}
			_, err22 := enviarComandoSSH(host.Ip, memoryCommand, config)
			if err22 != nil {
				log.Println("Error al realizar la actualizaciòn de la memoria", err22)
				return "Error al realizar la actualizaciòn de la memoria"
			}
			//Actualiza la RAM de la MV
			_, err2 := db.Exec("UPDATE maquina_virtual set ram = ? WHERE NOMBRE = ?", strconv.Itoa(specs.Ram), specs.Nombre)
			if err2 != nil {
				log.Println("Error al realizar la actualizaciòn de la memoria en la base de datos", err2)
				return "Error al realizar la actualizaciòn de la memoria en la base de datos"
			}
			fmt.Println("Se modificò con èxito la RAM")
		}
	}
	return "Modificaciones realizadas con èxito"
}

/* Funciòn que permite enviar el comando PowerOff para apagar una màquina virtual
@nameVM Paràmetro que contiene el nombre de la màquina virtual a apagar
@clientIP Paràmetro que contiene la direcciòn IP del cliente desde el cual se realiza la solicitud
*/

func apagarMV(nameVM string, clientIP string) string {

	//Obtiene la màquina vitual a apagar
	maquinaVirtual, err := getVM(nameVM)
	if err != nil {
		log.Println("Error al obtener la MV:", err)
		return "Error al obtener la MV"
	}
	//Obtiene el host en el cual està alojada la MV
	host, err1 := getHost(maquinaVirtual.Host_id)
	if err1 != nil {
		log.Println("Error al obtener el host:", err1)
		return "Error al obtener el host"
	}
	//Configura la conexiòn SSH con el host
	config, err2 := configurarSSH(host.Hostname, *privateKeyPath)
	if err2 != nil {
		log.Println("Error al configurar SSH:", err2)
		return "Error al configurar SSH"
	}
	//Variable que contiene el estado de la MV (Encendida o apagada)
	running, err3 := isRunning(nameVM, host.Ip, config)
	if err3 != nil {
		log.Println("Error al obtener el estado de la MV:", err3)
		return "Error al obtener el estado de la MV"
	}

	if !running { //En caso de que la MV estè apagada, entonces se invoca el mètodo para encenderla
		startVM(nameVM, clientIP)
	} else {

		//Comando para apagar la màquina virtual
		powerOffCommand := "VBoxManage controlvm " + "\"" + nameVM + "\"" + " poweroff"

		fmt.Println("Apagando màquina " + nameVM + "...")
		//Actualza el estado de la MV en la base de datos
		_, err4 := db.Exec("UPDATE maquina_virtual set estado = 'Procesando' WHERE NOMBRE = ?", nameVM)
		if err4 != nil {
			log.Println("Error al realizar la actualizaciòn del estado", err4)
			return "Error al realizar la actualizaciòn del estado"
		}
		//Envìa el comando para apagar la MV a travès de un ACPI
		_, err5 := enviarComandoSSH(host.Ip, powerOffCommand, config)
		if err5 != nil {
			log.Println("Error al enviar el comando para apagar la MV:", err5)
			return "Error al enviar el comando para apagar la MV"
		}
		// Establece un temporizador de espera máximo de 5 minutos
		maxEspera := time.Now().Add(5 * time.Minute)

		// Espera hasta que la máquina esté apagada o haya pasado el tiempo máximo de espera
		for time.Now().Before(maxEspera) {
			status, err6 := isRunning(nameVM, host.Ip, config)
			if err6 != nil {
				log.Println("Error al obtener el estado de la MV:", err6)
				return "Error al obtener el estado de la MV"
			}
			if !status {
				break
			}
			// Espera un 1 segundo antes de volver a verificar el estado de la màquina
			time.Sleep(1 * time.Second)
		}

		//Consulta si la MV està encendida
		status, err7 := isRunning(nameVM, host.Ip, config)
		if err7 != nil {
			log.Println("Error al obtener el estado de la MV:", err7)
			return "Error al obtener el estado de la MV"
		}
		if status {
			_, err8 := enviarComandoSSH(host.Ip, powerOffCommand, config) //Envìa el comando para apagar la MV a travès de un Power Off
			if err8 != nil {
				log.Println("Error al enviar el comando para apagar la MV:", err8)
				return "Error al enviar el comando para apagar la MV"
			}
		}
		//Actualiza el estado de la MV en la base de datos
		_, err9 := db.Exec("UPDATE maquina_virtual set estado = 'Apagado', ip = '' WHERE NOMBRE = ?", nameVM)
		if err9 != nil {
			log.Println("Error al realizar la actualizaciòn del estado", err9)
			return "Error al realizar la actualizaciòn del estado"
		}

		fmt.Println("Màquina apagada con èxito")
	}
	return ""
}

/*Funciòn que se encarga de gestionar la cola de solicitudes para la gestiòn de màquinas virtuales
 */
func checkManagementQueueChanges() {
	for {
		mu.Lock()
		currentQueueSize := managementQueue.Queue.Len()
		mu.Unlock()

		if currentQueueSize > 0 {
			mu.Lock()
			firstElement := managementQueue.Queue.Front()
			data, dataPresent := firstElement.Value.(map[string]interface{})
			mu.Unlock()

			if !dataPresent {
				fmt.Println("No se pudo procesar la solicitud")
				mu.Lock()
				managementQueue.Queue.Remove(firstElement)
				mu.Unlock()
				continue
			}

			tipoSolicitud, _ := data["tipo_solicitud"].(string)

			switch strings.ToLower(tipoSolicitud) {
			case "modify":
				specsMap, _ := data["specifications"].(map[string]interface{})
				specsJSON, err := json.Marshal(specsMap)
				if err != nil {
					fmt.Println("Error al serializar las especificaciones:", err)
					mu.Lock()
					managementQueue.Queue.Remove(firstElement)
					mu.Unlock()
					continue
				}

				var specifications Maquina_virtual
				err = json.Unmarshal(specsJSON, &specifications)
				if err != nil {
					fmt.Println("Error al deserializar las especificaciones:", err)
					mu.Lock()
					managementQueue.Queue.Remove(firstElement)
					mu.Unlock()
					continue
				}

				go modifyVM(specifications)

			case "delete":
				nameVM, _ := data["nombreVM"].(string)
				go deleteVM(nameVM)

			case "start":
				nameVM, _ := data["nombreVM"].(string)
				clientIP, _ := data["clientIP"].(string)
				go startVM(nameVM, clientIP)

			case "stop":
				nameVM, _ := data["nombreVM"].(string)
				clientIP, _ := data["clientIP"].(string)
				go apagarMV(nameVM, clientIP)

			default:
				fmt.Println("Tipo de solicitud no válido:", tipoSolicitud)
			}

			mu.Lock()
			managementQueue.Queue.Remove(firstElement)
			mu.Unlock()
		}

		time.Sleep(1 * time.Second) //Espera 1 segundo para volver a verificar la cola
	}
}

/* Funciòn que permite enviar los comandos necesarios para eliminar una màquina virtual
@nameVM Paràmetro que contiene el nombre de la màquina virtual a eliminar
*/

func deleteVM(nameVM string) string {

	//Obtiene el objeto "maquina_virtual"
	maquinaVirtual, err := getVM(nameVM)
	if err != nil {
		log.Println("Error al obtener la MV:", err)
		return "Error al obtener  la MV"
	}
	//Obtiene el host en el cual està alojada la MV
	host, err1 := getHost(maquinaVirtual.Host_id)
	if err1 != nil {
		log.Println("Error al obtener el host:", err)
		return "Error al obtener el host"
	}
	//Configura la conexiòn SSH con el host
	config, err2 := configurarSSH(host.Hostname, *privateKeyPath)
	if err2 != nil {
		log.Println("Error al configurar SSH:", err2)
		return "Error al configurar SSH"
	}

	//Comando para desconectar el disco de la MV
	disconnectCommand := "VBoxManage storageattach " + "\"" + nameVM + "\"" + " --storagectl hardisk --port 0 --device 0 --medium none"

	//Comando para eliminar la MV
	deleteCommand := "VBoxManage unregistervm " + "\"" + nameVM + "\"" + " --delete"

	//Variable que contiene el estado de la MV (Encendida o apagada)
	running, err3 := isRunning(nameVM, host.Ip, config)
	if err3 != nil {
		log.Println("Error al obtener el estado de la MV:", err3)
		return "Error al obtener el estado de la MV"
	}
	if running {
		fmt.Println("Debe apagar la màquina para eliminarla")
		return "Debe apagar la màquina para eliminarla"

	} else {
		//Envìa el comando para desconectar el disco de la MV
		_, err4 := enviarComandoSSH(host.Ip, disconnectCommand, config)
		if err4 != nil {
			log.Println("Error al desconectar el disco de la MV:", err4)
			return "Error al desconectar el disco de la MV"
		}
		//Envìa el comando para eliminar la MV del host
		_, err5 := enviarComandoSSH(host.Ip, deleteCommand, config)
		if err5 != nil {
			log.Println("Error al eliminar la MV:", err5)
			return "Error al eliminar la MV"
		}
		//Elimina la màquina virtual de la base de datos
		err6 := db.QueryRow("DELETE FROM maquina_virtual WHERE NOMBRE = ?", nameVM)
		if err6 == nil {
			log.Println("Error al eliminar el registro de la base de datos: ", err6)
			return "Error al eliminar el registro de la base de datos"
		}
		//Calcula los recursos usados del host, descontando los recursos liberados por la MV eliminada
		ram_host_usada := host.Ram_usada - maquinaVirtual.Ram
		cpu_host_usada := host.Cpu_usada - maquinaVirtual.Cpu
		//Actualiza los recursos usados del host en la base de datos
		err7 := db.QueryRow("UPDATE host set ram_usada = ?, cpu_usada = ? WHERE id = ?", ram_host_usada, cpu_host_usada, host.Id)
		if err7 == nil {
			log.Println("Error al actualizar los recursos usados del host en la base de datos: ", err7)
			return "Error al actualizar los recursos usados del host en la base de datos"
		}
	}
	fmt.Println("Màquina eliminada correctamente")
	return "Màquina eliminada correctamente"
}

/*
Funciòn que permite iniciar una màquina virtual en modo "headless", lo que indica que se inicia en segundo plano
para que el usuario de la màquina fìsica no se vea afectado
@nameVM Paràmetro que contiene el nombre de la màquina virtual a encender
*/

func startVM(nameVM string, clientIP string) string {

	//Obtiene el objeto "maquina_virtual"
	maquinaVirtual, err := getVM(nameVM)
	if err != nil {
		log.Println("Error al obtener la MV:", err)
		return "Error al obtener la MV"
	}
	//Obtiene el host en el cual està alojada la MV
	host, err1 := getHost(maquinaVirtual.Host_id)
	if err1 != nil {
		log.Println("Error al obtener el host:", err1)
		return "Error al obtener el host"
	}
	//Configura la conexiòn SSH con el host
	config, err2 := configurarSSH(host.Hostname, *privateKeyPath)
	if err2 != nil {
		log.Println("Error al configurar SSH:", err2)
		return "Error al configurar SSH"
	}

	//Variable que contiene el estado de la MV (Encendida o apagada)
	running, err3 := isRunning(nameVM, host.Ip, config)
	if err3 != nil {
		log.Println("Error al obtener el estado de la MV:", err3)
		return "Error al obtener el estado de la MV"
	}

	if running {
		apagarMV(nameVM, clientIP) //En caso de que la MV ya estè encendida, entonces se invoca el mètodo para apagar la MV
		return ""
	} else {
		fmt.Println("Encendiendo la màquina " + nameVM + "...")

		// Comando para encender la máquina virtual en segundo planto
		startVMHeadlessCommand := "VBoxManage startvm " + "\"" + nameVM + "\"" + " --type headless"

		//Comnado para encender la màquina virtual con GUI
		startVMGUICommand := "VBoxManage startvm " + "\"" + nameVM + "\""

		_, er := isAHostIp(clientIP) //Verifica si la solicitud se està realizando desde un host registrado en la BD
		if er == nil {
			//Envìa el comando para encender la MV con GUI
			_, err4 := enviarComandoSSH(host.Ip, startVMGUICommand, config)
			if err4 != nil {
				log.Println("Error al enviar el comando para encender la MV:", err4)
				return "Error al enviar el comando para encender la MV"
			}
		} else {
			//Envìa el comando para encender la MV en segundo plano
			_, err4 := enviarComandoSSH(host.Ip, startVMHeadlessCommand, config)
			if err4 != nil {
				log.Println("Error al enviar el comando para encender la MV:", err4)
				return "Error al enviar el comando para encender la MV"
			}
		}

		fmt.Println("Obteniendo direcciòn IP de la màquina " + nameVM + "...")
		//Actualiza el estado de la MV en la base de datos
		_, err5 := db.Exec("UPDATE maquina_virtual set estado = 'Procesando' WHERE NOMBRE = ?", nameVM)
		if err5 != nil {
			log.Println("Error al realizar la actualizaciòn del estado", err5)
			return "Error al realizar la actualizaciòn del estado"
		}
		// Espera 10 segundos para que la máquina virtual inicie
		time.Sleep(10 * time.Second)

		//Comando para obtener la dirección IP de la máquina virtual
		getIpCommand := "VBoxManage guestproperty get " + "\"" + nameVM + "\"" + " /VirtualBox/GuestInfo/Net/0/V4/IP"
		//Comando para reiniciar la MV
		rebootCommand := "VBoxManage controlvm " + "\"" + nameVM + "\"" + " reset"

		var ipAddress string

		// Establece un temporizador de espera máximo de 2 minutos
		maxEspera := time.Now().Add(2 * time.Minute)
		restarted := false

		for ipAddress == "" || ipAddress == "No value set!" || strings.HasPrefix(strings.TrimSpace(ipAddress), "169") {
			if time.Now().Before(maxEspera) {
				if ipAddress == "No value set!" {
					time.Sleep(5 * time.Second) // Espera 5 segundos antes de intentar nuevamente
					fmt.Println("Obteniendo dirección IP de la màquina " + nameVM + "...")
				}
				//Envìa el comando para obtener la IP
				ipAddress, _ = enviarComandoSSH(host.Ip, getIpCommand, config)

				ipAddress = strings.TrimSpace(ipAddress) //Elimina espacios en blanco al final de la cadena
				ipParts := strings.Split(ipAddress, ":")
				if len(ipParts) > 1 {
					ipParts := strings.Split(ipParts[1], ".")
					if strings.TrimSpace(ipParts[0]) == "169" {
						ipAddress = strings.TrimSpace(ipParts[0])
						time.Sleep(5 * time.Second) // Espera 5 segundos antes de intentar nuevamente
						fmt.Println("Obteniendo dirección IP de la màquina " + nameVM + "...")
					}
				}

			} else {
				if restarted {
					log.Println("No se logrò obtener la direcciòn IP de la màquina: " + nameVM)
					//Actualiza el estado de la MV en la base de datos
					_, err9 := db.Exec("UPDATE maquina_virtual set estado = 'Apagado' WHERE NOMBRE = ?", nameVM)
					if err9 != nil {
						log.Println("Error al realizar la actualizaciòn del estado", err9)
						return "Error al realizar la actualizaciòn del estado"
					}
					return "No se logrò obtener la direcciòn IP, por favor contacte al administrador"
				}
				//Envìa el comando para reiniciar la MV
				reboot, error := enviarComandoSSH(host.Ip, rebootCommand, config)
				if error != nil {
					log.Println("Error al reinciar la MV:", reboot)
					return "Error al reinciar la MV"
				}
				fmt.Println("Reiniciando la màquina: " + nameVM)
				maxEspera = time.Now().Add(2 * time.Minute) //Agrega dos minutos de tiempo màximo para obtener la IP cuando se reincia la MV
				restarted = true
			}
		}

		//Almacena solo el valor de la IP quitàndole el texto "Value:"
		ipAddress = strings.TrimPrefix(ipAddress, "Value:")
		ipAddress = strings.TrimSpace(ipAddress)
		//Actualiza el estado de la MV en la base de datos
		_, err9 := db.Exec("UPDATE maquina_virtual set estado = 'Encendido' WHERE NOMBRE = ?", nameVM)
		if err9 != nil {
			log.Println("Error al realizar la actualizaciòn del estado", err9)
			return "Error al realizar la actualizaciòn del estado"
		}
		//Actualiza la direcciòn IP de la MV en la base de datos
		_, err10 := db.Exec("UPDATE maquina_virtual set ip = ? WHERE NOMBRE = ?", ipAddress, nameVM)
		if err10 != nil {
			log.Println("Error al realizar la actualizaciòn de la IP", err10)
			return "Error al realizar la actualizaciòn de la IP"
		}
		fmt.Println("Màquina encendida, la direcciòn IP es: " + ipAddress)
		return ipAddress
	}
}

/*
Funciòn que contiene el algoritmo de asignaciòn tipo aleatorio. Se encarga de escoger un host de la base de datos al azar
Return Retorna el host seleccionado.
*/
func selectHost() (Host, error) {

	var host Host
	// Consulta para contar el número de registros en la tabla "host"
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM host").Scan(&count)
	if err != nil {
		log.Println("Error al realizar la consulta: " + err.Error())
		return host, err
	}

	// Genera un número aleatorio dentro del rango de registros
	rand.New(rand.NewSource(time.Now().Unix())) // Seed para generar números aleatorios diferentes en cada ejecución
	randomIndex := rand.Intn(count)

	// Consulta para seleccionar un registro aleatorio de la tabla "host"
	err = db.QueryRow("SELECT * FROM host ORDER BY RAND() LIMIT 1 OFFSET ?", randomIndex).Scan(&host.Id, &host.Nombre, &host.Mac, &host.Ip, &host.Hostname, &host.Ram_total, &host.Cpu_total, &host.Almacenamiento_total, &host.Ram_usada, &host.Cpu_usada, &host.Almacenamiento_usado, &host.Adaptador_red, &host.Estado, &host.Ruta_llave_ssh_pub, &host.Sistema_operativo, &host.Distribucion_sistema_operativo)
	if err != nil {
		log.Println("Error al realizar la consulta sql: ", err)
		return host, err
	}

	// Imprime el registro aleatorio seleccionado
	fmt.Printf("Registro aleatorio seleccionado: ")
	fmt.Printf("ID: %d, Nombre: %s, IP: %s\n", host.Id, host.Nombre, host.Ip)

	return host, nil
}

/*
Funciòn que permite conocer si ya existe o no una màquina virtual en la base de datos con el nombre proporcionado.
@nameVM Paràmetro que representa el nombre de la màquina virtual a buscar
@Return Retorna true si ya existe una MV con ese nombre, o false en caso contrario
*/
func existVM(nameVM string) (bool, error) {

	var existe bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM maquina_virtual WHERE nombre = ?)", nameVM).Scan(&existe)
	if err != nil {
		if err == sql.ErrNoRows {
			existe = false
		} else {
			log.Println("Error al realizar la consulta: ", err)
			return existe, err
		}
	}
	return existe, err
}

/*
Funciòn que permite obtener un host dado su identificador ùnico
@idHost Paràmetro que representa el identificador ùnico del host a buscar
@Return Retorna el host encontrado
*/
func getHost(idHost int) (Host, error) {

	var host Host
	err := db.QueryRow("SELECT * FROM host WHERE id = ?", idHost).Scan(&host.Id, &host.Nombre, &host.Mac, &host.Ip, &host.Hostname, &host.Ram_total, &host.Cpu_total, &host.Almacenamiento_total, &host.Ram_usada, &host.Cpu_usada, &host.Almacenamiento_usado, &host.Adaptador_red, &host.Estado, &host.Ruta_llave_ssh_pub, &host.Sistema_operativo, &host.Distribucion_sistema_operativo)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("No se encontró el host con el nombre especificado.")
		} else {
			log.Println("Error al realizar la consulta: ", err)
		}
		return host, err
	}
	return host, nil
}

/*
Funciòn que permite obtener una màquina virtual dado su nombre
@nameVM Paràmetro que representa el nombre de la màquina virtual a buscar
@Retorna la màquina virtual en caso de que exista en la base de datos
*/
func getVM(nameVM string) (Maquina_virtual, error) {
	var maquinaVirtual Maquina_virtual

	var fechaCreacionStr string
	err := db.QueryRow("SELECT * FROM maquina_virtual WHERE nombre = ?", nameVM).Scan(
		&maquinaVirtual.Uuid, &maquinaVirtual.Nombre, &maquinaVirtual.Ram,
		&maquinaVirtual.Cpu, &maquinaVirtual.Ip, &maquinaVirtual.Estado,
		&maquinaVirtual.Hostname, &maquinaVirtual.Persona_email,
		&maquinaVirtual.Host_id, &maquinaVirtual.Disco_id,
		&fechaCreacionStr)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("No se encontró la máquina virtual con el nombre especificado.")
		} else {
			log.Println("Hubo un error al realizar la consulta:", err)
		}
		return maquinaVirtual, err
	}

	fechaCreacion, err := time.Parse("2006-01-02 15:04:05", fechaCreacionStr)
	if err != nil {
		log.Println("Error al parsear la fecha de creación:", err)
		return maquinaVirtual, err
	}
	maquinaVirtual.Fecha_creacion = fechaCreacion

	return maquinaVirtual, nil
}

/*
Funciòn que permite obtener un usuario dado su identificador ùnico, es decir, su email
@email Paràmetro que representa el email del usuario a buscar
@Return Retorna el usuario (Persona) en caso de que exista un usuario con ese email
*/
func getUser(email string) (Persona, error) {

	var persona Persona
	err := db.QueryRow("SELECT * FROM persona WHERE email = ?", email).Scan(&persona.Email, &persona.Nombre, &persona.Apellido, &persona.Contrasenia, &persona.Rol)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("No se encontrò un usuario con el email especificado")
		} else {
			log.Println("Hubo un error al realizar la consulta: " + err.Error())
		}
		return persona, err
	}
	return persona, nil
}

/*
Funciòn que permite obtener un disco que cumpla con los paràmetros especificados
@sistema_operativo Paràmetro que representa el tipo de sistema operativo que debe tener el disco
@distribucion_sistema_operativo Paràmetro que representa la distribuciòn del sistema operativo
@id_host Paràmetro que representa el identificador ùnico del host en el cual se està buscando el disco
@Return Retorna el disco en caso de que exista y cumpla con las condiciones mencionadas anterormente
*/
func getDisk(sistema_operativo string, distribucion_sistema_operativo string, id_host int) (Disco, error) {

	var disco Disco
	//err := db.QueryRow("Select * from disco where sistema_operativo = ? and distribucion_sistema_operativo =? and host_id = ?", sistema_operativo, distribucion_sistema_operativo, id_host).Scan(&disco.Id, &disco.Nombre, &disco.Ruta_ubicacion, &disco.Sistema_operativo, &disco.Distribucion_sistema_operativo, &disco.arquitectura, &disco.Host_id)
	err := db.QueryRow("Select * from disco where sistema_operativo = ? and distribucion_sistema_operativo =? and host_id = ?", sistema_operativo, distribucion_sistema_operativo, id_host).Scan(&disco.Id, &disco.Nombre, &disco.Ruta_ubicacion, &disco.Sistema_operativo, &disco.Distribucion_sistema_operativo, &disco.arquitectura, &disco.Host_id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("No se encontrò un disco: " + sistema_operativo + " " + distribucion_sistema_operativo)
		} else {
			log.Println("Hubo un error al realizar la consulta: " + err.Error())
		}
		return disco, err
	}
	return disco, nil
}

/*
Funciòn que permite validar si un host tiene los recursos (CPU y RAM) que se estàn solicitando
@cpuRequerida Paràmetro que representa al cantidad de CPU requerida en el host
@ramRequerida Paràmetro que representa la cantidad de memoria RAM requerdida en el host
@host Paràmetro que representa el host en el cual se quiere realizar la validaciòn
@Return Retorna true en caso de que el host tenga libre los recursos solicitados, o false, en caso contrario
*/
func validarDisponibilidadRecursosHost(cpuRequerida int, ramRequerida int, host Host) bool {

	recursosDisponibles := false

	var cpuNecesitada int
	cpuDisponible := float64(host.Cpu_total) * 0.75 //Obtiene el 75% de la cpu total del host

	if cpuRequerida != 0 {
		cpuNecesitada = cpuRequerida + host.Cpu_usada
	}

	var ramNecesitada int
	ramDisponible := float64(host.Ram_total) * 0.75 //Obtiene el 75% de la ram total del host

	if ramRequerida != 0 {
		ramNecesitada = ramRequerida + host.Ram_usada
	}

	if cpuNecesitada != 0 && cpuNecesitada < int(cpuDisponible) {
		recursosDisponibles = true
	}
	if ramNecesitada != 0 && ramNecesitada < int(ramDisponible) {
		recursosDisponibles = true
	}
	return recursosDisponibles
}

/*
Funciòn que permite consultar el catàlogo de servicios de la plataforma
@Return Retorna un arreglo con los catàlogos disponibles de la plataforma
*/
func consultCatalog() ([]Catalogo, error) {

	var catalogo Catalogo
	var listaCatalogo []Catalogo

	query := "SELECT c.id, c.nombre, c.ram, c.cpu, d.sistema_operativo, d.distribucion_sistema_operativo, d.arquitectura FROM catalogo_disco cd JOIN catalogo c ON cd.catalogo_id = c.id JOIN disco d ON cd.disco_id = d.id"
	rows, err := db.Query(query)
	if err != nil {
		log.Println("Error al realizar la consulta del catàlogo en la base de datos")
		return listaCatalogo, err
	}
	defer rows.Close()

	for rows.Next() {

		if err := rows.Scan(&catalogo.Id, &catalogo.Nombre, &catalogo.Ram, &catalogo.Cpu, &catalogo.Sistema_operativo, &catalogo.Distribucion_sistema_operativo, &catalogo.Arquitectura); err != nil {
			log.Println("Error al obtener la fila")
			continue
		}
		listaCatalogo = append(listaCatalogo, catalogo)
	}

	if err := rows.Err(); err != nil {
		log.Println("Error al obtener el catàlogo de màquinas virtuales")
		return listaCatalogo, err
	}

	if len(listaCatalogo) == 0 {
		fmt.Println("El catàlogo està vacìo")
	}

	return listaCatalogo, nil
}

/*
Funciòn que genera una cadena alfanumèrica aleatoria
@length Paràmetro que contiene la longitud de la cadena a generar.
@Return Retorna la cadena aleatoria generada
*/
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// seededRand utiliza un generador de números aleatorios con una semilla basada en el tiempo actual en nanosegundos.
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		// Selecciona un carácter aleatorio del conjunto de caracteres
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

/*
Funciòn que se encarga de generar un correo aleatorio para las cuentas de lo sinvitados las cuales son temporales
*/

func generateRandomEmail() string {

	email := generateRandomString(5) + "@temp.com"
	return email
}

/*
Funciòn que se encarga de crear cuentas temporales para usuarios invitados
Cuando se crea la cuenta e ingresa a la base de datos, se encarga de invocar la funciòn para crear una màquina virtual temporal.

@clientIP Paràmetro que contiene la direcciòn IP desde la cual se està realizando la solicitud de crear la cuenta temporal
*/

func createTempAccount(clientIP string, distribucion_SO string) string {
	var persona Persona

	persona.Nombre = "Usuario"
	persona.Apellido = "Invitado"
	persona.Email = generateRandomEmail()
	persona.Contrasenia = "GuestUqcloud"
	persona.Rol = "Invitado"

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(persona.Contrasenia), bcrypt.DefaultCost)
	if err != nil {
		log.Println("Error al encriptar la contraseña:", err)
		return ""
	}

	query := "INSERT INTO persona (nombre, apellido, email, contrasenia, rol) VALUES ( ?, ?, ?, ?, ?);"

	//Consulta en la base de datos si el usuario existe
	_, err1 := db.Exec(query, persona.Nombre, persona.Apellido, persona.Email, hashedPassword, persona.Rol)
	if err1 != nil {
		log.Println("Hubo un error al registrar el usuario en la base de datos", err1)
	}

	createTempVM(persona.Email, clientIP, distribucion_SO)

	return persona.Email
}

/*
Funciòn que permite crear màquina virtuales temporales para los usuarios con rol "invitado"
Esta funciòn crea las especificaciones para crear una màquina virtual con recursos mìnimos
finalmente encola la solicitud de creaciòn

@email Paràmetro que contiene el email del usuario al cual le va a pertencer la MV
@clientIP Paràmetro que contiene la direcciòn IP desde la cual se està generando la peticiòn
*/

func createTempVM(email string, clientIP string, distribucion_SO string) {

	maquina_virtual := Maquina_virtual{
		Nombre:                         "Guest",
		Sistema_operativo:              "Linux",
		Distribucion_sistema_operativo: distribucion_SO,
		Ram:                            1024,
		Cpu:                            2,
		Persona_email:                  email,
	}

	payload := map[string]interface{}{
		"specifications": maquina_virtual,
		"clientIP":       clientIP,
	}

	jsonData, _ := json.Marshal(payload) //Se codifica en formato JSON

	var decodedPayload map[string]interface{}
	err := json.Unmarshal(jsonData, &decodedPayload) //Se decodifica para meterlo en la cola
	if err != nil {
		fmt.Println("Error al decodificar el JSON:", err)
		// Manejar el error según tus necesidades
		return
	}

	// Encola la peticiòn
	mu.Lock()
	maquina_virtualesQueue.Queue.PushBack(decodedPayload)
	mu.Unlock()
}

/*
Funciòn que dada una direcciòn IP permite conocer si pertenece o no a un host registrado en la base de datos.

@ip Paràmetro que contiene la direcciòn IP a consultar

@Return Retorna el host en caso de que la IP estè en la base de datos
*/
func isAHostIp(ip string) (Host, error) {

	var host Host
	err := db.QueryRow("SELECT * FROM host WHERE ip = ?", ip).Scan(&host.Id, &host.Nombre, &host.Mac, &host.Ip, &host.Hostname,
		&host.Ram_total, &host.Cpu_total, &host.Almacenamiento_total, &host.Ram_usada, &host.Cpu_usada,
		&host.Almacenamiento_usado, &host.Adaptador_red, &host.Estado, &host.Ruta_llave_ssh_pub, &host.Sistema_operativo,
		&host.Distribucion_sistema_operativo)
	if err != nil {
		if err == sql.ErrNoRows {
			return host, err
		}
		return host, err // Otro error de la base de datos
	}
	return host, nil // IP encontrada en la base de datos, devuelve el objeto Host correspondiente
}

/*
Funciòn que permite conocer las màquinas virtuales que tiene creadas un usuario ò todas las màquinas de la plataforma si es un administrador
@persona Paràmetro que representa un usuario, al cual se le van a buscar las màquinas que le pertenece
@return Retorna un arreglo con las màquinas que le pertenecen al usuario.
*/

func consultMachines(persona Persona) ([]Maquina_virtual, error) {

	email := persona.Email
	var query string
	var rows *sql.Rows
	var err error

	if persona.Rol == "Administrador" {
		//Consulta todas las màquinas virtuales de la base de datos
		query = "SELECT m.nombre, m.ram, m.cpu, m.ip, m.estado, d.sistema_operativo, d.distribucion_sistema_operativo, m.hostname FROM maquina_virtual as m INNER JOIN disco as d on m.disco_id = d.id"
		rows, err = db.Query(query)
	} else {
		//Consulta las màquinas virtuales de un usuario en la base de datos
		query = "SELECT m.nombre, m.ram, m.cpu, m.ip, m.estado, d.sistema_operativo, d.distribucion_sistema_operativo, m.hostname FROM maquina_virtual as m INNER JOIN disco as d on m.disco_id = d.id WHERE m.persona_email = ?"
		rows, err = db.Query(query, email)
	}

	var machines []Maquina_virtual

	if err != nil {
		log.Println("Error al realizar la consulta de màquinas en la BD", err)
		return machines, err
	}
	defer rows.Close()

	for rows.Next() {
		var machine Maquina_virtual
		if err := rows.Scan(&machine.Nombre, &machine.Ram, &machine.Cpu, &machine.Ip, &machine.Estado, &machine.Sistema_operativo, &machine.Distribucion_sistema_operativo, &machine.Hostname); err != nil {
			// Manejar el error al escanear la fila
			continue
		}
		machines = append(machines, machine)
	}

	if err := rows.Err(); err != nil {
		log.Println("Error al iterar sobre las filas ", err)
		return machines, err
	}

	if len(machines) == 0 {
		// No se encontraron máquinas virtuales para el usuario
		return machines, errors.New("no Machines Found")
	}
	return machines, nil
}

func consultHosts() ([]Host, error) {

	var query string
	var rows *sql.Rows
	var err error

	query = "SELECT id, nombre from host"
	rows, err = db.Query(query)

	var hosts []Host

	if err != nil {
		log.Println("Error al realizar la consulta de màquinas en la BD", err)
		return hosts, err
	}
	defer rows.Close()

	for rows.Next() {
		var host Host
		if err := rows.Scan(&host.Id, &host.Nombre); err != nil {
			// Manejar el error al escanear la fila
			continue
		}
		hosts = append(hosts, host)
	}

	if err := rows.Err(); err != nil {
		log.Println("Error al iterar sobre las filas ", err)
		return hosts, err
	}

	if len(hosts) == 0 {
		// No se encontraron máquinas virtuales para el usuario
		return hosts, errors.New("no Machines Found")
	}
	return hosts, nil
}

/*
Funciòn que verifica el tiempo de creaciòn de las màquinas de los usuarios invitados con el fin de determinar si se ha pasado o no del tiempo lìmite (2.5horas)
En caso de que una màquina se haya pasado del tiempo, se procederà a eliminarla.
*/

func checkMachineTime() {

	// Obtiene todas las máquinas virtuales de la base de datos
	maquinas, err := getGuestMachines()
	if err != nil {
		log.Println("Error al obtener las máquinas virtuales:", err)
		return
	}

	// Obtiene la hora actual
	horaActual := time.Now().UTC()

	for _, maquina := range maquinas {
		// Calcula la diferencia de tiempo entre la hora actual y la fecha de creación de la máquina
		diferencia := horaActual.Sub(maquina.Fecha_creacion)

		// Verifica si la máquina ha excedido su tiempo de duración, en este caso: 2horas 20minutos
		if diferencia > (2*time.Hour + 20*time.Minute) {

			//Obtiene el host en el cual està alojada la MV
			host, err1 := getHost(maquina.Host_id)
			if err1 != nil {
				log.Println("Error al obtener el host:", err)
				return
			}
			//Configura la conexiòn SSH con el host
			config, err2 := configurarSSH(host.Hostname, *privateKeyPath)
			if err2 != nil {
				log.Println("Error al configurar SSH:", err2)
				return
			}

			//Variable que contiene el estado de la MV (Encendida o apagada)
			running, err3 := isRunning(maquina.Nombre, host.Ip, config)
			if err3 != nil {
				log.Println("Error al obtener el estado de la MV:", err3)
				return
			}
			if running {
				apagarMV(maquina.Nombre, "")
			}
			deleteVM(maquina.Nombre)
		}
	}
}

/*
Funciòn que permite eliminar una cuenta de un usuario de la base de datos
@email Paràmetro que contiene el email del usuario a eliminar
*/

func deleteAccount(email string) {

	//Elimina la cuenta de la base de datos
	err := db.QueryRow("DELETE FROM persona WHERE email = ?", email)
	if err == nil {
		log.Println("Error al eliminar el registro de la base de datos: ", err)
	}
}

/*
Funciòn que permite obtener todas las màquinas virtuales creadas en la plataforma por los usuarios con rol invitado
@return Retorna un arreglo con todas las màquinas encontradas
*/

func getGuestMachines() ([]Maquina_virtual, error) {
	var maquinas []Maquina_virtual

	query := "SELECT m.nombre, m.fecha_creacion, m.host_id, m.persona_email FROM maquina_virtual m JOIN persona p ON m.persona_email = p.email WHERE p.rol = ?;"

	rows, err := db.Query(query, "Invitado")
	if err != nil {
		log.Println("Error al consultar las máquinas de los invitados:", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var machine Maquina_virtual
		var fechaCreacionStr string

		if err := rows.Scan(&machine.Nombre, &fechaCreacionStr, &machine.Host_id, &machine.Persona_email); err != nil {
			log.Println("Error al escanear la fila:", err)
			continue
		}

		// Convierte la cadena de fecha y hora a un tipo de datos time.Time
		fechaCreacion, err := time.Parse("2006-01-02 15:04:05", fechaCreacionStr)
		if err != nil {
			log.Println("Error al convertir la fecha y hora:", err)
			continue
		}

		// Asigna la fecha y hora convertida a la estructura Maquina_virtual
		machine.Fecha_creacion = fechaCreacion

		maquinas = append(maquinas, machine)
	}

	if err := rows.Err(); err != nil {
		log.Println("Error al iterar sobre las filas:", err)
		return nil, err
	}

	return maquinas, nil
}

/*
Funciòn que permite conocer el total de màquianas virtuales que tiene creadas un usuario
@email Paràmetro que contiene el email del usuario al cual se le va a contar las mpaquinas que tiene creadas
@return retorna un entero con el nùmero de màquinas creadas
*/

func countUserMachinesCreated(email string) (int, error) {

	//Obtiene la cantidad total de hosts que hay en la base de datos
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM maquina_virtual where persona_email = ?", email).Scan(&count)
	if err != nil {
		log.Println("Error al contar las màquinas del usuario que hay en la base de datos: " + err.Error())
		return 0, err
	}

	return count, nil
}

/*
Funciòn que se encarga de obtener diversas mètricas para el monitoreo de la plataforma
@return Retorna un mapa con los valores obtenidos en las consultas realizadas a la base de datos
*/

func getMetrics() (map[string]interface{}, error) {

	var metricas map[string]interface{}

	// Inicializar el mapa
	metricas = make(map[string]interface{})

	//Obtiene la cantidad total de màquinas virtuales que hay en la base de datos
	var total_maquinas_creadas int
	err := db.QueryRow("SELECT COALESCE(COUNT(*),0) FROM maquina_virtual").Scan(&total_maquinas_creadas)
	if err != nil {
		log.Println("Error al contar las màquinas creadas hay en la base de datos: " + err.Error())
		return nil, err
	}

	//Obtiene la cantidad total de màquinas virtuales encendidas que hay en la plataforma
	var total_maquinas_encendidas int
	err1 := db.QueryRow("SELECT COALESCE(COUNT(*),0) FROM maquina_virtual where estado = 'Encendido'").Scan(&total_maquinas_encendidas)
	if err1 != nil {
		log.Println("Error al contar las màquinas encendidas que hay en la plataforma: " + err1.Error())
		return nil, err1
	}

	//Obtiene la cantidad total de usuarios registradas en la base de datos
	var total_usuarios int
	err2 := db.QueryRow("SELECT COALESCE(COUNT(*),0) FROM persona").Scan(&total_usuarios)
	if err2 != nil {
		log.Println("Error al contar los usuarios totales registrados: " + err2.Error())
		return nil, err2
	}

	//Obtiene la cantidad total de usuarios con rol "estudiante"
	var total_estudiantes int
	err3 := db.QueryRow("SELECT COALESCE(COUNT(*),0) FROM persona where rol = 'Estudiante'").Scan(&total_estudiantes)
	if err3 != nil {
		log.Println("Error al contar los usuarios con rol estudiante: " + err3.Error())
		return nil, err3
	}

	//Obtiene la cantidad total de usuarios con rol "invitado"
	var total_invitados int
	err4 := db.QueryRow("SELECT COALESCE(COUNT(*),0) FROM persona where rol = 'Invitado'").Scan(&total_invitados)
	if err4 != nil {
		log.Println("Error al contar las màquinas encendidas que hay en la plataforma: " + err4.Error())
		return nil, err4
	}

	//Obtiene la cantidad total de memoria RAM que tiene la plataforma
	var total_RAM int
	err5 := db.QueryRow("SELECT COALESCE(SUM(ram_total),0) AS total_ram FROM host;").Scan(&total_RAM)
	if err5 != nil {
		log.Println("Error al contar el total de memoria RAM que tiene disponible la plataforma: " + err5.Error())
		return nil, err5
	}

	//Obtiene la cantidad total de memoria RAM que estàn usando las màquinas virtuales encendidas
	var total_RAM_usada int
	err6 := db.QueryRow("SELECT COALESCE(SUM(ram),0) AS total_ram_usada FROM maquina_virtual ;").Scan(&total_RAM_usada)
	if err6 != nil {
		log.Println("Error al contar el total de memoria RAM que estàn usando las màquinas encendidas: " + err6.Error())
		return nil, err6
	}

	//Obtiene la cantidad total de CPU que tiene la plataforma
	var total_CPU int
	err7 := db.QueryRow("SELECT COALESCE(SUM(cpu_total),0) AS total_cpu FROM host;").Scan(&total_CPU)
	if err7 != nil {
		log.Println("Error al contar el total de CPU que tiene disponible la plataforma: " + err7.Error())
		return nil, err7
	}

	//Obtiene la cantidad total de CPU que estàn usando las màquinas virtuales encendidas
	var total_CPU_usada int
	err8 := db.QueryRow("SELECT COALESCE(SUM(cpu),0) AS total_cpu_usada FROM maquina_virtual ;").Scan(&total_CPU_usada)
	if err8 != nil {
		log.Println("Error al contar el total de CPU que estàn usando las màquinas encendidas: " + err8.Error())
		return nil, err8
	}

	metricas["total_maquinas_creadas"] = total_maquinas_creadas
	metricas["total_maquinas_encendidas"] = total_maquinas_encendidas
	metricas["total_usuarios"] = total_usuarios
	metricas["total_estudiantes"] = total_estudiantes
	metricas["total_invitados"] = total_invitados
	metricas["total_RAM"] = total_RAM / 1024             //Se divide por 1024 para pasar de Mb a Gb
	metricas["total_RAM_usada"] = total_RAM_usada / 1024 //Se divide por 1024 para pasar de Mb a Gb
	metricas["total_CPU"] = total_CPU
	metricas["total_CPU_usada"] = total_CPU_usada

	return metricas, nil
}
