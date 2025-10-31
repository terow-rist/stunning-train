package main

import (
	"flag"
	"fmt"
	"os"
	"ride-hail/internal/cli"
	"time"
)

func main() {
	var (
		userID = flag.String("user-id", "", "UUID of the user (subject)")
		role   = flag.String("role", "PASSENGER", "User role: PASSENGER | DRIVER | ADMIN")
		secret = flag.String("secret", "", "JWT HMAC secret (HS256)")
	)
	flag.Parse()

	if *userID == "" || *secret == "" {
		fmt.Fprintln(os.Stderr, "usage: token --user-id=<uuid> --role=PASSENGER --secret='<secret>' [--ttl=2h]")
		os.Exit(2)
	}

	token, claims, err := cli.GenerateUserToken(*secret, *userID, *role)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Println("TOKEN:")
	fmt.Println(token)
	fmt.Println("\nCLAIMS:")
	fmt.Printf("  sub:  %s\n", claims.Subject)
	fmt.Printf("  role: %s\n", claims.Role)
	fmt.Printf("  iat:  %s\n", claims.IssuedAt.Time.UTC().Format(time.RFC3339))
	fmt.Printf("  exp:  %s\n", claims.ExpiresAt.Time.UTC().Format(time.RFC3339))
}
