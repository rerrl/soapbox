package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/gorilla/mux"
	"github.com/sendgrid/sendgrid-go"

	signinwithapple "github.com/Timothylock/go-signin-with-apple/apple"
	"github.com/soapboxsocial/soapbox/pkg/account"
	"github.com/soapboxsocial/soapbox/pkg/activeusers"
	"github.com/soapboxsocial/soapbox/pkg/analytics"
	"github.com/soapboxsocial/soapbox/pkg/apple"
	"github.com/soapboxsocial/soapbox/pkg/blocks"
	"github.com/soapboxsocial/soapbox/pkg/conf"
	"github.com/soapboxsocial/soapbox/pkg/devices"
	"github.com/soapboxsocial/soapbox/pkg/followers"
	httputil "github.com/soapboxsocial/soapbox/pkg/http"
	"github.com/soapboxsocial/soapbox/pkg/http/middlewares"
	"github.com/soapboxsocial/soapbox/pkg/images"
	"github.com/soapboxsocial/soapbox/pkg/linkedaccounts"
	"github.com/soapboxsocial/soapbox/pkg/login"
	"github.com/soapboxsocial/soapbox/pkg/mail"
	"github.com/soapboxsocial/soapbox/pkg/me"
	"github.com/soapboxsocial/soapbox/pkg/minis"
	"github.com/soapboxsocial/soapbox/pkg/notifications"
	"github.com/soapboxsocial/soapbox/pkg/pubsub"

	// "github.com/soapboxsocial/soapbox/pkg/recommendations/follows"
	"github.com/soapboxsocial/soapbox/pkg/redis"
	"github.com/soapboxsocial/soapbox/pkg/rooms/pb"
	"github.com/soapboxsocial/soapbox/pkg/search"
	"github.com/soapboxsocial/soapbox/pkg/sessions"
	"github.com/soapboxsocial/soapbox/pkg/sql"
	"github.com/soapboxsocial/soapbox/pkg/stories"
	"github.com/soapboxsocial/soapbox/pkg/users"
	"google.golang.org/grpc"
)

type Conf struct {
	// Twitter struct {
	// 	Key    string `mapstructure:"key"`
	// 	Secret string `mapstructure:"secret"`
	// } `mapstructure:"twitter"`
	Sendgrid struct {
		Key string `mapstructure:"key"`
	} `mapstructure:"sendgrid"`
	CDN struct {
		Images  string `mapstructure:"images"`
		Stories string `mapstructure:"stories"`
	} `mapstructure:"cdn"`
	Apple  conf.AppleConf    `mapstructure:"apple"`
	Redis  conf.RedisConf    `mapstructure:"redis"`
	DB     conf.PostgresConf `mapstructure:"db"`
	GRPC   conf.AddrConf     `mapstructure:"grpc"`
	Listen conf.AddrConf     `mapstructure:"listen"`
	Login  login.Config      `mapstructure:"login"`
	Minis  []struct {
		Key string `mapstructure:"key"`
		ID  int    `mapstructure:"id"`
	} `mapstructure:"mini"`
}

func parse() (*Conf, error) {
	var file string
	flag.StringVar(&file, "c", "config.toml", "config file")
	flag.Parse()

	fmt.Printf("file: %s\n\n", file)

	config := &Conf{}
	err := conf.Load(file, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func main() {
	config, err := parse()
	if err != nil {
		log.Fatalf("failed to parse config err: %v", err)
	}
	fmt.Printf("config: %+v\n\n", config)

	rdb := redis.NewRedis(config.Redis)
	fmt.Printf("rdb: %+v\n\n", rdb)

	_, err = rdb.Ping(rdb.Context()).Result()
	if err != nil {
		log.Fatalf("failed to ping redis: %s", err)
	}

	queue := pubsub.NewQueue(rdb)
	fmt.Printf("queue: %+v\n\n", queue)

	db, err := sql.Open(config.DB)
	if err != nil {
		log.Fatalf("failed to open db: %s", err)
	}
	fmt.Printf("db: %+v\n\n", db)

	s := sessions.NewSessionManager(rdb)
	ub := users.NewBackend(db)
	fb := followers.NewFollowersBackend(db)
	ns := notifications.NewStorage(rdb)
	fmt.Printf("s: %+v\n\n", s)
	fmt.Printf("ub: %+v\n\n", ub)
	fmt.Printf("fb: %+v\n\n", fb)
	fmt.Printf("ns: %+v\n\n", ns)

	client, err := elasticsearch.NewDefaultClient()
	if err != nil {
		panic(err)
	}
	fmt.Printf("client: %+v\n\n", client)

	devicesBackend := devices.NewBackend(db)
	fmt.Printf("devicesBackend: %+v\n\n", devicesBackend)

	amw := middlewares.NewAuthenticationMiddleware(s)
	fmt.Printf("amw: %+v\n\n", amw)

	r := mux.NewRouter()
	fmt.Printf("r: %+v\n\n", r)

	// health check endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK")
	}).Methods("GET")

	r.MethodNotAllowedHandler = http.HandlerFunc(httputil.NotAllowedHandler)
	r.NotFoundHandler = http.HandlerFunc(httputil.NotFoundHandler)

	ib := images.NewImagesBackend(config.CDN.Images)
	ms := mail.NewMailService(sendgrid.NewSendClient(config.Sendgrid.Key))
	fmt.Printf("ib: %+v\n\n", ib)
	fmt.Printf("ms: %+v\n\n", ms)

	loginState := login.NewStateManager(rdb)
	fmt.Printf("loginState: %+v\n\n", loginState)

	secret, err := ioutil.ReadFile(config.Apple.Path)
	if err != nil {
		panic(err)
	}
	fmt.Printf("secret: %+v\n\n", secret)

	appleClient, err := apple.NewSignInWithAppleAppValidation(
		signinwithapple.New(),
		config.Apple.TeamID,
		config.Apple.Bundle,
		config.Apple.KeyID,
		string(secret),
	)
	fmt.Printf("appleClient: %+v\n\n", appleClient)

	if err != nil {
		panic(err)
	}

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", config.GRPC.Host, config.GRPC.Port), grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("conn: %+v\n\n", conn)

	defer conn.Close()

	roomService := pb.NewRoomServiceClient(conn)
	fmt.Printf("roomService: %+v\n\n", roomService)

	loginEndpoints := login.NewEndpoint(ub, loginState, s, ms, ib, queue, appleClient, roomService, config.Login)
	loginRouter := loginEndpoints.Router()
	fmt.Printf("loginRouter: %+v\n\n", loginRouter)
	mount(r, "/v1/login", loginRouter)

	storiesBackend := stories.NewBackend(db)
	fmt.Printf("storiesBackend: %+v\n\n", storiesBackend)

	usersEndpoints := users.NewEndpoint(
		ub,
		fb,
		s,
		ib,
		queue,
		storiesBackend,
	)
	usersRouter := usersEndpoints.Router()
	usersRouter.Use(amw.Middleware)
	mount(r, "/v1/users", usersRouter)

	storiesEndpoint := stories.NewEndpoint(storiesBackend, stories.NewFileBackend(config.CDN.Stories), queue)
	storiesRouter := storiesEndpoint.Router()
	storiesRouter.Use(amw.Middleware)
	mount(r, "/v1/stories", storiesRouter)

	devicesEndpoint := devices.NewEndpoint(devicesBackend)
	devicesRoutes := devicesEndpoint.Router()
	devicesRoutes.Use(amw.Middleware)
	mount(r, "/v1/devices", devicesRoutes)

	accountEndpoint := account.NewEndpoint(account.NewBackend(db), queue, s)
	accountRouter := accountEndpoint.Router()
	accountRouter.Use(amw.Middleware)
	mount(r, "/v1/account", accountRouter)

	blocksBackend := blocks.NewBackend(db)
	blocksEndpoint := blocks.NewEndpoint(blocksBackend)
	blocksRouter := blocksEndpoint.Router()
	blocksRouter.Use(amw.Middleware)
	mount(r, "/v1/blocks", blocksRouter)

	// twitter oauth config
	// oauth := oauth1.NewConfig(
	// 	config.Twitter.Key,
	// 	config.Twitter.Secret,
	// )
	// fmt.Printf("oauth: %+v\n\n", oauth)

	pb := linkedaccounts.NewLinkedAccountsBackend(db)

	meEndpoint := me.NewEndpoint(ub, ns, pb, storiesBackend, queue, activeusers.NewBackend(db), notifications.NewSettings(db))
	meRoutes := meEndpoint.Router()

	meRoutes.Use(amw.Middleware)
	mount(r, "/v1/me", meRoutes)

	searchEndpoint := search.NewEndpoint(client)
	searchRouter := searchEndpoint.Router()
	searchRouter.Use(amw.Middleware)
	mount(r, "/v1/search", searchRouter)

	minisBackend := minis.NewBackend(db)

	keys := make(minis.AuthKeys)
	for _, m := range config.Minis {
		keys[m.Key] = m.ID
	}

	minisEndpoint := minis.NewEndpoint(minisBackend, amw, keys)

	minisRouter := minisEndpoint.Router()
	mount(r, "/v1/minis", minisRouter)

	analyticsBackend := analytics.NewBackend(db)
	analyticsEndpoint := analytics.NewEndpoint(analyticsBackend)
	analyticsRouter := analyticsEndpoint.Router()
	analyticsRouter.Use(amw.Middleware)
	mount(r, "/v1/analytics", analyticsRouter)

	err = http.ListenAndServe(fmt.Sprintf(":%d", config.Listen.Port), httputil.CORS(r))
	if err != nil {
		log.Print(err)
	}

	fmt.Println("Server started")
}

func mount(r *mux.Router, path string, handler http.Handler) {
	r.PathPrefix(path).Handler(
		http.StripPrefix(
			strings.TrimSuffix(path, "/"),
			AddSlashForRoot(handler),
		),
	)
}

// AddSlashForRoot adds a slash if the path is the root path.
// This is necessary for our subrouters where there may be a root.
func AddSlashForRoot(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// @TODO MAYBE ENSURE SUFFIX DOESN'T ALREADY EXIST?
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}

		next.ServeHTTP(w, r)
	})
}
