package main

import (
	"container/list"
	"database/sql"
	"encoding/json"
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
@Usuario Representa el usuario de la cuenta con el cual puede iniciar sesiìn
@Contrasenia Representa la contraseña de la cuenta
*/
type Persona struct {
	Nombre      string
	Apellido    string
	Email       string
	Usuario     string
	Contrasenia string
	Rol         string
}

/*
Estructura de datos tipo JSOn que contiene los datos de una màquina virtual
@Uuid Representa el uuid de una màqina virtual, el cual es un identificador ùnico
@Nombre Representa el nombre de la MV
@Sistema_operativo Representa el tipo de sistema operativo
@Memoria Representa la cantidad de memoria RAM de la MV
@Cpu Representa la cantidad de unidades de procesamiento de la MV
@Estado Representa el estado de la MV (Apagada, Encendida)
@Hostname Representa el nombre del host de la MV
@Ip Representa la direcciòn Ip de la MV
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
}

type Maquina_virtualQueue struct {
	sync.Mutex
	Queue *list.List
}

/*
--------------------------------------------------------------------
Estructura de datos tipo JSON que contiene los campos de un host
@Id Representa el identificador ùnico del host
@Nombre Representa el nombre del host
@Mac Representa la direcciòn fìsica del host
@Memoria Representa la cantidad de memoria RAM que tiene el host
@Cpu Representa la cantidad de unidades de procesamiento que tiene el host
@Adaptador_red Representa el nombre del adaptador de red
@Almacenamiento_total Representa el total de espacio de almacenamiento que tiene el host
@Estado Representa el estado del host (Disponible Fuera de servicio)
@Sistema_operativo Representa el tipo de sistema operativo del host
@Ruta_disco_multi Representa la ubiaciòn del disco multiconexiòn
@Ruta_llave_ssh Representa la ubiaciòn de la llave ssh pùblica
@Hostname Representa el nombre del host
@Ip Representa la direcciòn Ip del host
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
------------------------------------------------------
Estructura de datos tipo JSON que contiene los campos para representar una MV del catàlogo
@Nombre Representa el nombre de la MV
@Memoria Representa la cantidad de memoria RAM de la MV
@Cpu Representa la cantidad de unidades de procesamiento de la MV
@Sistema_operativo Representa el tipo de sistema operativo de la Mv
*/
type Catalogo struct {
	Id     int
	Nombre string
	Ram    int
	Cpu    int
}

// -------------------------
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

	// Función que verifica la cola de cuentas constantemente.
	go checkManagementQueueChanges()

	// Inicia el servidor HTTP en el puerto 8081.
	fmt.Println("Servidor escuchando en el puerto 8081...")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		fmt.Println("Error al iniciar el servidor:", err)
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
		var maquina_virtual Maquina_virtual
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&maquina_virtual); err != nil {
			http.Error(w, "Error al decodificar JSON de especificaciones", http.StatusBadRequest)
			return
		}

		fmt.Println(maquina_virtual)

		// Encola las especificaciones.
		mu.Lock()
		maquina_virtualesQueue.Queue.PushBack(maquina_virtual)
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
			response := map[string]bool{"loginCorrecto": false}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(response)
		} else if err != nil {
			panic(err.Error())
		} else {
			response := map[string]bool{"loginCorrecto": true}
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

		//Consulta en la base de datos si el usuario existe
		a, err := db.Exec(query, persona.Nombre, persona.Apellido, persona.Email, hashedPassword, "Estudiante")
		fmt.Println(a)
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

	//Endpoint para peticiones de inicio de sesiòn
	http.HandleFunc("/json/consultMachine", func(w http.ResponseWriter, r *http.Request) {
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

		query := "SELECT m.nombre, m.ram, m.cpu, m.ip, m.estado, d.sistema_operativo, d.distribucion_sistema_operativo FROM maquina_virtual as m INNER JOIN disco as d on m.disco_id = d.id WHERE m.persona_email = ?"
		rows, err := db.Query(query, persona.Email)
		if err != nil {
			// Manejar el error
			return
		}
		defer rows.Close()

		var machines []Maquina_virtual
		for rows.Next() {
			var machine Maquina_virtual
			if err := rows.Scan(&machine.Nombre, &machine.Ram, &machine.Cpu, &machine.Ip, &machine.Estado, &machine.Sistema_operativo, &machine.Distribucion_sistema_operativo); err != nil {
				// Manejar el error al escanear la fila
				continue
			}
			machines = append(machines, machine)
		}

		if err := rows.Err(); err != nil {
			// Manejar el error al iterar a través de las filas
			return
		}

		if len(machines) == 0 {
			// No se encontraron máquinas virtuales para el usuario
			return
		}

		// Respondemos con la lista de máquinas virtuales en formato JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(machines)

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

	//------------------------------------------------------------------------------------------------------------
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

		// Envía una respuesta al cliente.
		response := map[string]string{"mensaje": "Mensaje JSON para encender MV recibido correctamente"}
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
			maquina_virtualesQueue.Queue.Remove(firstElement)
			mu.Unlock()

			crateVM(firstElement.Value.(Maquina_virtual))

			printMaquinaVirtual(firstElement.Value.(Maquina_virtual), true)
		}

		// Espera un segundo antes de verificar nuevamente.
		time.Sleep(1 * time.Second)
	}
}

/*
	Funciòn que se encarga de imprimir peticiones

@specs Este paràmetro contiene las especificaciones de la màquina virtual gestionada
@isCreateVM Variable de tipo booleana que si es verdadera significa que la peticiòn es de crear una màquina virtual. En caso contrario
indica que la peticiòn es para modificar una màquina virtual
*/
func printSpecifications(specs Maquina_virtual, isCreateVM bool) {

	// Imprime las especificaciones recibidas.
	fmt.Printf("-------------------------\n")
	fmt.Printf("Nombre de la Máquina: %s\n", specs.Nombre)
	fmt.Printf("Sistema Operativo: %s\n", specs.Sistema_operativo)
	fmt.Printf("Distribuciòn SO: %s\n", specs.Distribucion_sistema_operativo)
	fmt.Printf("Memoria Requerida: %d Mb\n", specs.Ram)
	fmt.Printf("CPU Requerida: %d núcleos\n", specs.Cpu)

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

	// Imprime la salida del comando
	if strings.TrimSpace(string(output)) != "" {
		fmt.Println(string(output))
	}

	return string(output), nil
}

/*
	Esta funciòn permite enviar los comandos VBoxManage necesarios para crear una nueva màquina virtual

@spects Paràmetro que contiene la configuraciòn enviarda por el usuario para crear la MV
@hostIP Paràmetro que contiene la direcciòn IP del host--------------------------------------
@config Paràmetro que contiene la configuraciòn de la conexiòn SSH con el host
*/
func crateVM(specs Maquina_virtual) string {

	//obtiene el usuario que hace la peticiòn
	user, error := getUser(specs.Persona_email)
	if error != nil {
		log.Println("Error al obtener el usuario:", error)
		return "Error al obtener el usuario"
	}

	nameVM := specs.Nombre + "_" + user.Nombre

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

	//Selecciona un host al azar
	host, err10 := selectHost()
	if err10 != nil {
		log.Println("Error al seleccionar el host:", err10)
		return "Error al seleccionar el host"
	}

	disco, err20 := getDisk(specs.Sistema_operativo, specs.Distribucion_sistema_operativo, host.Id)
	if err20 != nil {
		log.Println("Error al obtener el disco:", err20)
		return "Error al obtener el disco"
	}

	// Procesa el primer elemento (en este caso, imprime las especificaciones).
	config, err := configurarSSH(host.Hostname, *privateKeyPath)
	if err != nil {
		log.Println("Error al configurar SSH:", err)
		return "Error al configurar la conexiòn SSH"
	}

	//Comando para crear una màquina virtual
	createVM := "VBoxManage createvm --name " + "\"" + nameVM + "\"" + " --ostype " + specs.Distribucion_sistema_operativo + "_" + strconv.Itoa(disco.arquitectura) + " --register"
	fmt.Println(createVM)
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

	//comando para poner el adaptador de red en modo puente (Bridge)
	redAdapterCommand := "VBoxManage modifyvm " + "\"" + nameVM + "\"" + " --nic1 bridged --bridgeadapter1 " + "\"" + host.Adaptador_red + "\""
	_, err6 := enviarComandoSSH(host.Ip, redAdapterCommand, config)
	if err6 != nil {
		log.Println("Error al ejecutar el comando para configurar el adaptador de red de la MV:", err6)
		return "Error al configurar el adaptador de red de la MV"
	}

	lines := strings.Split(string(uuid), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "UUID:") {
			uuid = strings.TrimPrefix(line, "UUID:")
		}
	}

	nuevaMaquinaVirtual := Maquina_virtual{
		Uuid:              uuid,
		Nombre:            nameVM,
		Sistema_operativo: specs.Sistema_operativo,
		Ram:               specs.Ram,
		Cpu:               specs.Cpu,
		Estado:            "Apagado",
		Hostname:          "uqcloud",
		Persona_email:     specs.Persona_email,
	}

	fmt.Println(specs)

	_, err7 := db.Exec("INSERT INTO maquina_virtual (uuid, nombre,  ram, cpu, ip, estado, hostname, persona_email, host_id, disco_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		nuevaMaquinaVirtual.Uuid, nuevaMaquinaVirtual.Nombre, nuevaMaquinaVirtual.Ram, nuevaMaquinaVirtual.Cpu,
		nuevaMaquinaVirtual.Ip, nuevaMaquinaVirtual.Estado, nuevaMaquinaVirtual.Hostname, nuevaMaquinaVirtual.Persona_email, host.Id, disco.Id)

	if err7 != nil {
		log.Println("Error al crear el registro en la base de datos:", err7)
		return "Error al crear el registro en la base de datos"
	}

	fmt.Println("Màquina virtual creada con èxito")
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
si la màquina esta encendida o apagada. En caso de que estè encendida, invoca la funciòn para apagar la màquina.
@specs Paràmetro que contiene las especificaciones a modificar en la màquina virtual
@hostIP Paràmetro que contiene la direcciòn ip del host en el cual està alojada la màquina virtual a modificar
*/

func modifyVM(specs Maquina_virtual) string {

	//Obtiene el objeto "maquina_virtual"
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

	if specs.Cpu != 0 {
		_, err11 := enviarComandoSSH(host.Ip, cpuCommand, config)
		if err11 != nil {
			log.Println("Error al realizar la actualizaciòn de la cpu", err11)
			return "Error al realizar la actualizaciòn de la cpu"
		}
		_, err1 := db.Exec("UPDATE maquina_virtual set cpu = ? WHERE NOMBRE = ?", strconv.Itoa(specs.Cpu), specs.Nombre)
		if err1 != nil {
			log.Println("Error al realizar la actualizaciòn de la cpu", err1)
			return "Error al realizar la actualizaciòn de la cpu"
		}
	}

	if specs.Ram != 0 {
		_, err22 := enviarComandoSSH(host.Ip, memoryCommand, config)
		if err22 != nil {
			log.Println("Error al realizar la actualizaciòn de la memoria", err22)
			return "Error al realizar la actualizaciòn de la memoria"
		}
		_, err2 := db.Exec("UPDATE maquina_virtual set ram = ? WHERE NOMBRE = ?", strconv.Itoa(specs.Ram), specs.Nombre)
		if err2 != nil {
			log.Println("Error al realizar la actualizaciòn de la memoria en la base de datos", err2)
			return "Error al realizar la actualizaciòn de la memoria en la base de datos"
		}
	}

	fmt.Println("Las modificaciones se realizaron correctamente")
	return "Modificaciones realizadas con èxito"
}

/* Funciòn que permite enviar los comandos necesarios para apagar una màquina virtual
@nameVM Paràmetro que contiene el nombre de la màquina virtual a apagar
@hostIP Paràmetro que contiene la direcciòn ip del host
*/

func apagarMV(nameVM string) string {

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

	if !running {
		startVM(nameVM)
	} else {
		//Comando para enviar señal de apagado a la MV esperando que los programas cierren correctamente
		acpiCommand := "VBoxManage controlvm " + "\"" + nameVM + "\"" + " acpipowerbutton"

		//Comando para apagar la màquina sin esperar que los programas cierren
		powerOffCommand := "VBoxManage controlvm " + "\"" + nameVM + "\"" + " poweroff"

		fmt.Println("Apagando màquina " + nameVM + "...")
		_, err4 := db.Exec("UPDATE maquina_virtual set estado = 'Procesando' WHERE NOMBRE = ?", nameVM)
		if err4 != nil {
			log.Println("Error al realizar la actualizaciòn del estado", err4)
			return "Error al realizar la actualizaciòn del estado"
		}

		_, err5 := enviarComandoSSH(host.Ip, acpiCommand, config)
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

		status, err7 := isRunning(nameVM, host.Ip, config)
		if err7 != nil {
			log.Println("Error al obtener el estado de la MV:", err7)
			return "Error al obtener el estado de la MV"
		}
		if status {
			_, err8 := enviarComandoSSH(host.Ip, powerOffCommand, config)
			if err8 != nil {
				log.Println("Error al enviar el comando para apagar la MV:", err8)
				return "Error al enviar el comando para apagar la MV"
			}
		}
		_, err9 := db.Exec("UPDATE maquina_virtual set estado = 'Apagado' WHERE NOMBRE = ?", nameVM)
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
		// Verifica si el tamaño de la cola de especificaciones ha cambiado.
		mu.Lock()
		currentQueueSize := managementQueue.Queue.Len()
		mu.Unlock()

		if currentQueueSize > 0 {
			// Imprime y elimina el primer elemento de la cola de especificaciones.
			mu.Lock()
			firstElement := managementQueue.Queue.Front()

			// Verifica que el primer elemento sea un mapa.
			data, dataPresent := firstElement.Value.(map[string]interface{})

			if !dataPresent {
				fmt.Println("No se pudo procesar la solicitud")
				mu.Unlock()
				return
			}

			// Obtiene el valor del campo "tipo_solicitud"
			tipoSolicitud, _ := data["tipo_solicitud"].(string)

			if strings.ToLower(tipoSolicitud) == "modify" {
				specsMap, _ := data["specifications"].(map[string]interface{})

				// Convierte el mapa de especificaciones a un objeto Specifications.
				specsJSON, err := json.Marshal(specsMap)
				if err != nil {
					fmt.Println("Error al serializar las especificaciones:", err)
					mu.Unlock()
					return
				}

				var specifications Maquina_virtual
				err = json.Unmarshal(specsJSON, &specifications)
				if err != nil {
					fmt.Println("Error al deserializar las especificaciones:", err)
					mu.Unlock()
					return
				}

				modifyVM(specifications)
			}

			//--------------------------------------------------------------------------------------
			if strings.ToLower(tipoSolicitud) == "delete" {

				// Obtiene el valor del campo "tipo_solicitud"
				nameVM, _ := data["nombreVM"].(string)

				deleteVM(nameVM)
			}

			if strings.ToLower(tipoSolicitud) == "start" {

				// Obtiene el valor del campo "tipo_solicitud"
				nameVM, _ := data["nombreVM"].(string)

				startVM(nameVM)
			}

			if strings.ToLower(tipoSolicitud) == "stop" {

				// Obtiene el valor del campo "tipo_solicitud"
				nameVM, _ := data["nombreVM"].(string)

				apagarMV(nameVM)
			}

			managementQueue.Queue.Remove(firstElement)
			mu.Unlock()
		}

		// Espera un segundo antes de verificar nuevamente.
		time.Sleep(1 * time.Second)
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

	config, err2 := configurarSSH(host.Hostname, *privateKeyPath)
	if err2 != nil {
		log.Println("Error al configurar SSH:", err2)
		return "Error al configurar SSH"
	}

	//Comando para desconectar el disco de la MV
	disconnectCommand := "VBoxManage storageattach " + "\"" + nameVM + "\"" + " --storagectl hardisk --port 0 --device 0 --medium none"
	fmt.Println(disconnectCommand)

	//Comando para eliminar la MV
	deleteCommand := "VBoxManage unregistervm " + "\"" + nameVM + "\"" + " --delete"
	fmt.Println(deleteCommand)

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
		_, err4 := enviarComandoSSH(host.Ip, disconnectCommand, config)
		if err4 != nil {
			log.Println("Error al desconectar el disco de la MV:", err4)
			return "Error al desconectar el disco de la MV"
		}

		_, err5 := enviarComandoSSH(host.Ip, deleteCommand, config)
		if err5 != nil {
			log.Println("Error al eliminar la MV:", err5)
			return "Error al eliminar la MV"
		}

		err6 := db.QueryRow("DELETE FROM maquina_virtual WHERE NOMBRE = ?", nameVM)
		if err6 == nil {
			log.Println("Error al eliminar el registro de la base de datos: ", err6)
			return "Error al eliminar el registro de la base de datos"
		}
	}

	fmt.Println("Màquina eliminada correctamente")
	return "Màquina eliminada correctamente"
}

/*
Funciòn que permite iniciar una màquina virtual en modo "headless", lo que indica que se inicia en segundo plano
para que el usuario de la màquina fìsica no se vea afectado
@specs Contiene las especificaciones de la maquina a encender
@hostIp Contiene la direcciòn Ip del host en el cual està alojada la MV
@return Retorna la direcciòn Ip de la màquina virtual
*/

func startVM(nameVM string) string {

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
		apagarMV(nameVM)
		return ""
	} else {
		fmt.Println("Encendiendo la màquina " + nameVM + "...")

		// Comando para encender la máquina virtual
		startVMCommand := "VBoxManage startvm " + "\"" + nameVM + "\"" + " --type headless"
		_, err4 := enviarComandoSSH(host.Ip, startVMCommand, config)
		if err4 != nil {
			log.Println("Error al enviar el comando para encender la MV:", err4)
			return "Error al enviar el comando para encender la MV"
		}

		fmt.Println("Obteniendo direcciòn IP...")
		_, err5 := db.Exec("UPDATE maquina_virtual set estado = 'Procesando' WHERE NOMBRE = ?", nameVM)
		if err5 != nil {
			log.Println("Error al realizar la actualizaciòn del estado", err5)
			return "Error al realizar la actualizaciòn del estado"
		}
		// Espera 10 segundos para que la máquina virtual inicie
		time.Sleep(10 * time.Second)

		// Obtiene la dirección IP de la máquina virtual después de que se inicie
		getIpCommand := "VBoxManage guestproperty get " + "\"" + nameVM + "\"" + " /VirtualBox/GuestInfo/Net/0/V4/IP"
		var ipAddress string
		ipAddress, err6 := enviarComandoSSH(host.Ip, getIpCommand, config)
		if err6 != nil {
			log.Println("Error al obtener la IP de la MV:", err6)
			return "Error al obtener la IP de la MV"
		}
		ipAddress = strings.TrimSpace(ipAddress)

		for ipAddress == "" || ipAddress == "No value set!" {

			if ipAddress == "No value set!" {
				time.Sleep(5 * time.Second) // Espera 5 segundos antes de intentar nuevamente
				fmt.Println("Obteniendo dirección IP...")
			}
			ipAddress, err6 = enviarComandoSSH(host.Ip, getIpCommand, config)
			if err6 != nil {
				log.Println("Error al obtener la IP de la MV:", err6)
				return "Error al obtener la IP de la MV"
			}
			ipAddress = strings.TrimSpace(ipAddress)
		}

		time.Sleep(5 * time.Second) // Espera 5 segundos antes de intentar nuevamente
		ipAddress, err8 := enviarComandoSSH(host.Ip, getIpCommand, config)
		if err8 != nil {
			log.Println("Error al enviar el comando para obtener la IP de la MV:", err8)
			return "Error al enviar el comando para obtener la IP de la MV"
		}

		//Almacena solo el valor de la IP quitàndole el texto "Value:"
		ipAddress = strings.TrimPrefix(ipAddress, "Value:")
		ipAddress = strings.TrimSpace(ipAddress)

		_, err9 := db.Exec("UPDATE maquina_virtual set estado = 'Encendido' WHERE NOMBRE = ?", nameVM)
		if err9 != nil {
			log.Println("Error al realizar la actualizaciòn del estado", err9)
			return "Error al realizar la actualizaciòn del estado"
		}
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

func getVM(nameVM string) (Maquina_virtual, error) {

	var maquinaVirtual Maquina_virtual
	err := db.QueryRow("SELECT * FROM maquina_virtual WHERE nombre = ?", nameVM).Scan(&maquinaVirtual.Uuid, &maquinaVirtual.Nombre, &maquinaVirtual.Ram, &maquinaVirtual.Cpu, &maquinaVirtual.Ip, &maquinaVirtual.Estado, &maquinaVirtual.Hostname, &maquinaVirtual.Persona_email, &maquinaVirtual.Host_id, &maquinaVirtual.Disco_id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("No se encontró la màquina virtual con el nombre especificado.")
		} else {
			log.Println("Hubo un error al realizar la consulta: ", err)
		}
		return maquinaVirtual, err
	}

	return maquinaVirtual, nil
}

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

func getDisk(sistema_operativo string, distribucion_sistema_operativo string, id_host int) (Disco, error) {

	var disco Disco
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
