package ecspresso_test

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic/types"
	"github.com/aws/smithy-go/middleware"
	"github.com/google/go-cmp/cmp"
	"github.com/kayac/ecspresso/v2"
)

func TestDecodeAuthorizationToken(t *testing.T) {
	tests := []struct {
		name         string
		token        string
		wantUser     string
		wantPassword string
		wantErr      bool
	}{
		{name: "valid", token: base64.StdEncoding.EncodeToString([]byte("AWS:pass:word")), wantUser: "AWS", wantPassword: "pass:word"},
		{name: "not base64", token: "!!!", wantErr: true},
		{name: "no colon", token: base64.StdEncoding.EncodeToString([]byte("AWS")), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, password, err := ecspresso.DecodeAuthorizationToken(tt.token)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if diff := cmp.Diff([]string{tt.wantUser, tt.wantPassword}, []string{user, password}); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestVerifierECRPublicCredentials(t *testing.T) {
	tests := []struct {
		name         string
		output       *ecrpublic.GetAuthorizationTokenOutput
		err          error
		wantUser     string
		wantPassword string
		wantErr      bool
	}{
		{
			name: "success",
			output: &ecrpublic.GetAuthorizationTokenOutput{
				AuthorizationData: &types.AuthorizationData{
					AuthorizationToken: aws.String(base64.StdEncoding.EncodeToString([]byte("AWS:secret"))),
				},
			},
			wantUser:     "AWS",
			wantPassword: "secret",
		},
		{
			name:    "api error",
			err:     errors.New("access denied"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			var regions []string
			cfg := aws.Config{
				Region:      "ap-northeast-1",
				Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
				APIOptions: []func(*middleware.Stack) error{
					func(stack *middleware.Stack) error {
						return stack.Finalize.Add(
							middleware.FinalizeMiddlewareFunc("test", func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								calls++
								regions = append(regions, awsmiddleware.GetRegion(ctx))
								if tt.err != nil {
									return middleware.FinalizeOutput{}, middleware.Metadata{}, tt.err
								}
								return middleware.FinalizeOutput{Result: tt.output}, middleware.Metadata{}, nil
							}),
							middleware.Before,
						)
					},
				},
			}
			v := ecspresso.NewVerifier(&cfg, &cfg, &ecspresso.VerifyOption{})
			for range 2 {
				user, password, err := v.ECRPublicCredentials(t.Context())
				if tt.wantErr {
					if err == nil {
						t.Fatal("expected error")
					}
					continue
				}
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				if diff := cmp.Diff([]string{tt.wantUser, tt.wantPassword}, []string{user, password}); diff != "" {
					t.Errorf("mismatch (-want +got):\n%s", diff)
				}
			}
			// the result is cached, including errors
			if calls != 1 {
				t.Errorf("GetAuthorizationToken called %d times, want 1", calls)
			}
			if diff := cmp.Diff([]string{"us-east-1"}, regions); diff != "" {
				t.Errorf("region mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
