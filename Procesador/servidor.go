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

	"golang.org/x/crypto/ssh"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// Variable que almacena la ruta de la llave privada ingresada por paametro cuando de ejecuta el programa
var privateKeyPath = flag.String("key", "", "Ruta de la llave privada SSH")

/*
Estructura tipo JSON que contiene los datos que ingresa el usuario para la creaciòn de una MV
@Name Este campo representa el nombre de la màquina virtual a crear
@OSType Este campo representa el tipo de sistema operativo de la màquina virtual
@Memory Este campo representa la cantidad de memoria RAM a asiganar a la MV
@CPU Este campo representa la cantidad de unidades de procesamiento de la MV
*/
type Specifications struct {
	Name   string `json:"nombre"`
	OSType string `json:"tipoSO"`
	Memory int    `json:"memoria"`
	CPU    int    `json:"cpu"`
}

/*
Estrucutura de datos tipo JSON que contiene los campos necesarios para iniciar sesiòn
@Username Este campo representa el nombre de usuario
@Password Este campo representa la contraseña
*/
type Account struct {
	Username string `json:"nombre"`
	Password string `json:"contrasenia"`
}

// Cola de especificaciones para la creaciòn de màquinas virtuales
type SpecificationsQueue struct {
	sync.Mutex
	Queue *list.List
}

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
	Uuid              string
	Nombre            string
	Sistema_operativo string
	Memoria           int
	Cpu               int
	Estado            string
	Hostname          string
	Ip                string
	Persona_email     string
	Host_id           string
}

/*
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
	Id                   int
	Nombre               string
	Mac                  string
	Memoria              int
	Cpu                  int
	Adaptador_red        string
	Almacenamiento_total int
	Estado               string
	Sistema_operativo    string
	Ruta_disco_multi     string
	Ruta_llave_ssh       string
	Hostname             string
	Ip                   string
}

/*
Estructura de datos tipo JSON que contiene los campos para representar una MV del catàlogo
@Nombre Representa el nombre de la MV
@Memoria Representa la cantidad de memoria RAM de la MV
@Cpu Representa la cantidad de unidades de procesamiento de la MV
@Sistema_operativo Representa el tipo de sistema operativo de la Mv
*/
type Catalogo struct {
	Nombre            string
	Memoria           int
	Cpu               int
	Sistema_operativo string
}

// Declaraciòn de variables globales
var (
	specificationsQueue SpecificationsQueue
	managementQueue     ManagementQueue
	mu                  sync.Mutex
	lastQueueSize       int
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
	go checkSpecificationsQueueChanges()

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
	specificationsQueue.Queue = list.New()
	managementQueue.Queue = list.New()

	//Endpoint para las peticiones de creaciòn de màquinas virtuales
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

	//Endpoint para peticiones de inicio de sesiòn
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
		query := "SELECT persona FROM persona WHERE usuario = ? AND contrasenia = ?"
		var resultUsername string

		//Consulta en la base de datos si el usuario existe
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
	//End point para eliminar màquinas virtuales
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
		nombre, nombrePresente := datos["nombre"].(string)
		ipHost, ipPresente := datos["ip"].(string)
		tipoSolicitud, tipoPresente := datos["tipo_solicitud"].(string)

		if !tipoPresente || tipoSolicitud != "delete" {
			http.Error(w, "El campo 'tipo_solicitud' debe ser 'delete'", http.StatusBadRequest)
			return
		}

		if !nombrePresente || !ipPresente || tipoPresente || nombre == "" || ipHost == "" || tipoSolicitud == "" {
			http.Error(w, "El tipo de solicitud, nombre de la máquina virtual y la IP del host son obligatorios", http.StatusBadRequest)
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

}

/* Funciòn que se encarga de gestionar la cola de peticiones para la creaciòn de màquinas virtuales
 */
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
			config, err := configurarSSH("jhoiner", *privateKeyPath)
			if err != nil {
				log.Fatal("Error al configurar SSH:", err)
				return
			}

			crateVM(firstElement.Value.(Specifications), config) //En el segundo argumento debe ir la ip del host en el cual se va a crear la VM

			printSpecifications(firstElement.Value.(Specifications), true)
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
func printSpecifications(specs Specifications, isCreateVM bool) {
	// Crea el comando en VirtualBox
	//comandCreate := "Vboxmanage createvm --name " + specs.Name + " --ostype " + specs.OSType
	//comandModify := "Vboxmanage modifyvm " + specs.Name + " --memory " + strconv.Itoa(specs.Memory) + " --vram 128"

	// Imprime las especificaciones recibidas.
	fmt.Printf("-------------------------\n")
	fmt.Printf("Nombre de la Máquina: %s\n", specs.Name)
	fmt.Printf("Sistema Operativo: %s\n", specs.OSType)
	fmt.Printf("Memoria Requerida: %d Mb\n", specs.Memory)
	fmt.Printf("CPU Requerida: %d núcleos\n", specs.CPU)

}

func printAccount(account Account) {
	// Imprime la cuenta recibida.
	fmt.Printf("-------------------------\n")
	fmt.Printf("Nombre de Usuario: %s\n", account.Username)
	fmt.Printf("Contraseña: %s\n", account.Password)
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
func enviarComandoSSH(host string, comando string, config *ssh.ClientConfig) (salida string) {

	//Establece la conexiòn SSH
	conn, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		log.Fatalf("Error al establecer la conexiòn SSH: %s", err)
	}
	defer conn.Close()

	//Crea una nueva sesiòn SSH
	session, err := conn.NewSession()
	if err != nil {
		log.Fatalf("Error al crear la sesiòn SSH: %s", err)
	}
	defer session.Close()

	//Ejecuta el comando remoto
	output, err := session.CombinedOutput(comando)
	if err != nil {
		log.Fatalf("Error al crear la sesiòn SSH: %s", err)
	}

	//Imprime la salida del comado
	fmt.Println(string(output))

	return string(output)
}

/*
	Esta funciòn permite enviar los comandos VBoxManage necesarios para crear una nueva màquina virtual

@spects Paràmetro que contiene la configuraciòn enviarda por el usuario para crear la MV
@hostIP Paràmetro que contiene la direcciòn IP del host--------------------------------------
@config Paràmetro que contiene la configuraciòn de la conexiòn SSH con el host
*/
func crateVM(specs Specifications, config *ssh.ClientConfig) {

	//Selecciona un host al azar
	host := selectHost()

	//Comando para crear una màquina virtual
	createVM := "VBoxManage createvm --name " + specs.Name + " --ostype " + specs.OSType + " --register"
	uuid := enviarComandoSSH(host.Ip, createVM, config)

	//Comando para asignar la memoria RAM a la MV
	memoryCommand := "VBoxManage modifyvm " + specs.Name + " --memory " + strconv.Itoa(specs.Memory)
	enviarComandoSSH(host.Ip, memoryCommand, config)

	//Comando para agregar el controlador de almacenamiento
	sctlCommand := "VBoxManage storagectl " + specs.Name + " --name hardisk --add sata"
	enviarComandoSSH(host.Ip, sctlCommand, config)

	//Comando para conectar el disco multiconexiòn a la MV
	fmt.Println("1")
	sattachCommand := "VBoxManage storageattach " + specs.Name + " --storagectl hardisk --port 0 --device 0 --type hdd --medium " + "\"" + host.Ruta_disco_multi + "\""
	enviarComandoSSH(host.Ip, sattachCommand, config)
	fmt.Println("2")

	//Comando para asignar las unidades de procesamiento
	cpuCommand := "VBoxManage modifyvm " + specs.Name + " --cpus " + strconv.Itoa(specs.CPU)
	enviarComandoSSH(host.Ip, cpuCommand, config)

	//comando para poner el adaptador de red en modo puente (Bridge)
	redAdapterCommand := "VBoxManage modifyvm " + specs.Name + " --nic1 bridged --bridgeadapter1 " + "\"" + host.Adaptador_red + "\""
	enviarComandoSSH(host.Ip, redAdapterCommand, config)

	lines := strings.Split(string(uuid), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "UUID:") {
			uuid = strings.TrimPrefix(line, "UUID:")
		}
	}

	nuevaMaquinaVirtual := Maquina_virtual{
		Uuid:              uuid,
		Nombre:            specs.Name,
		Sistema_operativo: specs.OSType,
		Memoria:           specs.Memory,
		Cpu:               specs.CPU,
		Estado:            "Apagada",
		Hostname:          "uqcloud",
	}

	_, err := db.Exec("INSERT INTO maquina_virtual (uuid, nombre, sistema_operativo, memoria, cpu, estado,persona_email, host_id, hostname, ip) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		nuevaMaquinaVirtual.Uuid, nuevaMaquinaVirtual.Nombre, nuevaMaquinaVirtual.Sistema_operativo, nuevaMaquinaVirtual.Memoria,
		nuevaMaquinaVirtual.Cpu, nuevaMaquinaVirtual.Estado, "jslopezd@uqvirtual.edu.co", host.Id, nuevaMaquinaVirtual.Hostname, nuevaMaquinaVirtual.Ip)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Màquina virtual creada con èxito")
}

/*
	Esta funciòn verifica si una màquina virtual està encendida

@nameVM Paràmetro que contiene el nombre de la màquina virtual a verificar
@hostIP Paràmetro que contiene la direcciòn Ip del host en el cual està la MV
@return Retorna true si la màquina està encendida o false en caso contrario
*/
func isRunning(nameVM string, hostIP string, config *ssh.ClientConfig) (running bool) {

	//Comando para saber el estado de una màquina virtual
	command := "VBoxManage showvminfo " + nameVM + " | findstr /C:\"State:\""
	running = false

	salida := enviarComandoSSH(hostIP, command, config)

	// Expresión regular para buscar el estado (running)
	regex := regexp.MustCompile(`State:\s+(running|powered off)`)
	matches := regex.FindStringSubmatch(salida)

	// matches[1] contendrá "running" o "powered off" dependiendo del estado
	if len(matches) > 1 {
		estado := matches[1]
		fmt.Println("Estado:", estado)
		if estado == "running" {
			running = true
		}
	}
	return running
}

/* Funciòn que contiene los comandos necesarios para modificar una màquina virtual. Primero verifica
si la màquina esta encendida o apagada. En caso de que estè encendida, invoca la funciòn para apagar la màquina.
@specs Paràmetro que contiene las especificaciones a modificar en la màquina virtual
@hostIP Paràmetro que contiene la direcciòn ip del host en el cual està alojada la màquina virtual a modificar
*/

func modifyVM(specs Specifications, hostIP string, config *ssh.ClientConfig) string {

	//Comando para modificar la memoria RAM a la MV
	memoryCommand := "VBoxManage modifyvm " + specs.Name + " --memory " + strconv.Itoa(specs.Memory)

	//Comando para modificar las unidades de procesamiento
	cpuCommand := "VBoxManage modifyvm " + specs.Name + " --cpus " + strconv.Itoa(specs.CPU)

	//Variable que contiene el estado de la MV (Encendida o apagada)
	running := isRunning(specs.Name, hostIP, config)

	if running {
		return "Para modificar la màquina primero debe apagarla"

	}

	if specs.CPU != 0 {
		enviarComandoSSH(hostIP, cpuCommand, config)
	}

	if specs.Memory != 0 {
		enviarComandoSSH(hostIP, memoryCommand, config)
	}

	fmt.Println("Las modificaciones se realizaron correctamente")
	return "Modificaciones realizadas con èxito"
}

/* Funciòn que permite enviar los comandos necesarios para apagar una màquina virtual
@nameVM Paràmetro que contiene el nombre de la màquina virtual a apagar
@hostIP Paràmetro que contiene la direcciòn ip del host
*/

func apagarMV(nameVM string, hostIP string, config *ssh.ClientConfig) {

	//Comando para enviar señal de apagado a la MV esperando que los programas cierren correctamente
	acpiCommand := "VBoxManage controlvm " + nameVM + " acpipowerbutton"

	//Comando para apagar la màquina sin esperar que los programas cierren
	powerOffCommand := "VBoxManage controlvm " + nameVM + " poweroff"

	enviarComandoSSH(hostIP, acpiCommand, config)

	// Establece un temporizador de espera máximo de 5 minutos
	maxEspera := time.Now().Add(5 * time.Minute)

	// Espera hasta que la máquina esté apagada o haya pasado el tiempo máximo de espera
	for time.Now().Before(maxEspera) {
		if !isRunning(nameVM, hostIP, config) {
			break
		}

		// Espera un 5 segundos antes de volver a verificar el estado de la màquina
		time.Sleep(5 * time.Second)
	}

	if isRunning(nameVM, hostIP, config) {
		enviarComandoSSH(hostIP, powerOffCommand, config)
	}

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

			config, err := configurarSSH("jhoiner", *privateKeyPath)
			if err != nil {
				log.Fatal("Error al configurar SSH:", err)
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

				var specifications Specifications
				err = json.Unmarshal(specsJSON, &specifications)
				if err != nil {
					fmt.Println("Error al deserializar las especificaciones:", err)
					mu.Unlock()
					return
				}

				modifyVM(specifications, "192.168.101.10", config)
			}

			//--------------------------------------------------------------------------------------
			if strings.ToLower(tipoSolicitud) == "delete" {

				// Obtiene el valor del campo "tipo_solicitud"
				nameVM, _ := data["nombreVM"].(string)

				deleteVM(nameVM)
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
	maquinaVirtual := getVM(nameVM)

	//Obtiene el host en el cual està alojada la MV
	host := getHost(maquinaVirtual.Host_id)

	config, err := configurarSSH(host.Hostname, *privateKeyPath)
	if err != nil {
		fmt.Println("Error al configurar SSH:", err)
	}

	//Comando para desconectar el disco de la MV
	disconnectCommand := "VBoxManage storageattach " + nameVM + " --storagectl hardisk --port 0 --device 0 --medium none"

	//Comando para eliminar la MV
	deleteCommand := "VBoxManage unregistervm " + nameVM + " --delete"

	//Variable que contiene el estado de la MV (Encendida o apagada)
	running := isRunning(nameVM, host.Ip, config)

	if running {
		return "Debe apagar la màquina primero"

	} else {
		enviarComandoSSH(host.Ip, disconnectCommand, config)
		enviarComandoSSH(host.Ip, deleteCommand, config)

		err := db.QueryRow("DELETE FROM maquina_virtual WHERE NOMBRE = ?", nameVM)
		if err != nil {
			fmt.Println("Error al eliminar el registro de la base de datos: ", err)
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

func startVM(specs Specifications, config *ssh.ClientConfig) string {

	//Obtiene el host en el cual està alojada la MV
	host := getHost(specs.Name)
	hostIP := host.Ip

	// Comando para encender la máquina virtual
	startVMCommand := "VBoxManage startvm " + specs.Name + " --type headless"
	enviarComandoSSH(hostIP, startVMCommand, config)

	// Espera 10 segundos para que la máquina virtual inicie
	time.Sleep(10 * time.Second)

	// Obtiene la dirección IP de la máquina virtual después de que se inicie
	getIpCommand := "VBoxManage guestproperty get " + specs.Name + " /VirtualBox/GuestInfo/Net/0/V4/IP"
	var ipAddress string

	for ipAddress == "" || ipAddress == "No value set!" {
		ipAddress = enviarComandoSSH(hostIP, getIpCommand, config)
		if ipAddress == "No value set!" {
			time.Sleep(5 * time.Second) // Espera 5 segundos antes de intentar nuevamente
		}

	}
	return ipAddress
}

/*
Funciòn que contiene el algoritmo de asignaciòn tipo aleatorio. Se encarga de escoger un host de la base de datos al azar
Return Retorna el host seleccionado.
*/

func selectHost() Host {

	// Consulta para contar el número de registros en la tabla "host"
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM host").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	// Genera un número aleatorio dentro del rango de registros
	rand.New(rand.NewSource(time.Now().Unix())) // Seed para generar números aleatorios diferentes en cada ejecución
	randomIndex := rand.Intn(count)

	// Consulta para seleccionar un registro aleatorio de la tabla "host"
	var host Host

	err = db.QueryRow("SELECT * FROM host LIMIT ?, 1", randomIndex).Scan(&host.Id, &host.Nombre, &host.Mac, &host.Memoria, &host.Cpu, &host.Adaptador_red, &host.Almacenamiento_total, &host.Estado, &host.Sistema_operativo, &host.Ruta_disco_multi, &host.Ruta_llave_ssh, &host.Hostname, &host.Ip)
	if err != nil {
		log.Fatal(err)
	}

	// Imprime el registro aleatorio seleccionado
	fmt.Printf("Registro aleatorio seleccionado:\n")
	fmt.Printf("ID: %d, Nombre: %s, MAC: %s, Memoria: %d, CPU: %d, Adaptador Red: %s, Estado: %s, SO: %s, Ruta Disco Multi: %s, Ruta Llave SSH: %s, Hostname: %s, IP: %s\n", host.Id, host.Nombre, host.Mac, host.Memoria, host.Cpu, host.Adaptador_red, host.Estado, host.Sistema_operativo, host.Ruta_disco_multi, host.Ruta_llave_ssh, host.Hostname, host.Ip)

	return host
}

func existVM(nameVM string) bool {

	var existe bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM maquinas_virtuales WHERE nombre = ?)", nameVM).Scan(&existe)
	if err != nil {
		log.Fatal(err)
	}

	if existe {
		return true
	}

	return false
}

func getHost(idHost string) Host {

	var host Host
	err := db.QueryRow("SELECT * FROM host WHERE id = ?", idHost).Scan(&host.Id, &host.Nombre, &host.Mac, &host.Memoria, &host.Cpu, &host.Adaptador_red, &host.Almacenamiento_total, &host.Estado, &host.Sistema_operativo, &host.Ruta_disco_multi, &host.Ruta_llave_ssh, &host.Hostname, &host.Ip)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("No se encontró el host con el nombre especificado.")
		} else {
			log.Fatal(err)
		}
	}

	return host

}

func getVM(nameVM string) Maquina_virtual {

	var maquinaVirtual Maquina_virtual
	fmt.Println("nombre vm" + nameVM)
	err := db.QueryRow("SELECT * FROM maquina_virtual WHERE nombre = ?", nameVM).Scan(&maquinaVirtual.Uuid, &maquinaVirtual.Nombre, &maquinaVirtual.Sistema_operativo, &maquinaVirtual.Memoria, &maquinaVirtual.Cpu, &maquinaVirtual.Estado, &maquinaVirtual.Persona_email, &maquinaVirtual.Host_id, &maquinaVirtual.Hostname, &maquinaVirtual.Ip)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("No se encontró la màquina virtual con el nombre especificado.")
		} else {
			log.Fatal(err)
		}
	}

	return maquinaVirtual
}
