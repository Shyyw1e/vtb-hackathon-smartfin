package cache_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/cache"
	"github.com/Shyyw1e/vtb-hackathon-smartfin/pkg/logger"
	"github.com/alicebob/miniredis/v2"
)

func init() {
	l := logger.InitLog("debug")
	l.SetOutput(io.Discard)
}
func TestSetGetJSON(t *testing.T) {
  mr, _ := miniredis.Run(); defer mr.Close()
  ctx := context.Background()
  s, err := cache.New(ctx, mr.Addr(), "", 0)
  if err != nil { t.Fatal(err) }

  type payload struct{ A string; N int }
  want := payload{"ok", 42}

  if err := s.SetJSON(ctx, "k", want, time.Minute); err != nil { t.Fatal(err) }
  var got payload
  found, err := s.GetJSON(ctx, "k", &got)
  if err != nil || !found { t.Fatalf("found=%v err=%v", found, err) }
  if got != want { t.Fatalf("mismatch: %+v", got) }
}
