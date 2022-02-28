package gateway

import (
	"bequest/insecure"
	"bequest/third_party"
	"context"
	"crypto/tls"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	api "bequest/proto"
)

func getOpenAPIHandler() http.Handler {
	if err := mime.AddExtensionType(".svg", "image/svg+xml"); err != nil {
		log.Fatalf("MIME svg: %v", err)
	}

	subFS, err := fs.Sub(third_party.OpenAPI, "OpenAPI")
	if err != nil {
		panic("couldn't create sub filesystem: " + err.Error())
	}
	return http.FileServer(http.FS(subFS))
}

func handleRoutingError(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, httpStatus int) {
	fmt.Println(httpStatus)
	runtime.DefaultRoutingErrorHandler(ctx, mux, marshaler, w, r, httpStatus)
	//if httpStatus != http.StatusMethodNotAllowed {
	//	runtime.DefaultRoutingErrorHandler(ctx, mux, marshaler, w, r, httpStatus)
	//	return
	//}

	//// Use HTTPStatusError to customize the DefaultHTTPErrorHandler status code
	//err := &runtime.HTTPStatusError{
	//	HTTPStatus: httpStatus,
	//	Err:        status.Error(codes.In, http.StatusText(httpStatus)),
	//}

	//runtime.DefaultHTTPErrorHandler(ctx, mux, marshaler, w , r, err)
}

func errHandler(c context.Context, s *runtime.ServeMux, m runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
	var newErr error
	if sterr := status.Convert(err); sterr.Code() == codes.AlreadyExists {
		newErr = status.Error(codes.InvalidArgument, sterr.Message())
	} else {
		newErr = err
	}

	runtime.DefaultHTTPErrorHandler(c, s, m, w, r, newErr)
}

func Run(dialAddr string) error {
	conn, err := grpc.DialContext(
		context.Background(),
		dialAddr,
		grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(insecure.CertPool, "")),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to dial server: %w", err)
	}

	gwmux := runtime.NewServeMux(
		runtime.WithRoutingErrorHandler(handleRoutingError),
		runtime.WithErrorHandler(errHandler),
	)
	err = api.RegisterKeyValueStoreHandler(context.Background(), gwmux, conn)
	if err != nil {
		return fmt.Errorf("failed to register gateway: %w", err)
	}

	oa := getOpenAPIHandler()

	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}
	gatewayAddr := "0.0.0.0:" + port
	gwServer := &http.Server{
		Addr: gatewayAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api") {
				gwmux.ServeHTTP(w, r)
				return
			}
			oa.ServeHTTP(w, r)
		}),
	}

	if strings.ToLower(os.Getenv("SERVE_HTTP")) == "true" {
		fmt.Println("Serving gRPC-Gateway and OpenAPI Documentation on http://", gatewayAddr)
		return fmt.Errorf("serving gRPC-Gateway server: %w", gwServer.ListenAndServe())
	}

	gwServer.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{insecure.Cert},
	}
	fmt.Println("Serving gRPC-Gateway and OpenAPI Documentation on https://", gatewayAddr)
	return fmt.Errorf("serving gRPC-Gateway server: %w", gwServer.ListenAndServeTLS("", ""))
}
