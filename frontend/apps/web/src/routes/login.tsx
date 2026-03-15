// frontend/apps/web/src/routes/login.tsx
import { useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";
import { Button } from "@frontend/ui/components/button";
import { Input } from "@frontend/ui/components/input";
import { Label } from "@frontend/ui/components/label";
import { Card, CardContent, CardHeader, CardTitle } from "@frontend/ui/components/card";

import { api, ApiError } from "@/lib/api";
import { setToken } from "@/lib/auth";

export const Route = createFileRoute("/login")({
  component: LoginPage,
});

function LoginPage() {
  const navigate = useNavigate();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    try {
      if (mode === "register") {
        await api.post("/auth/register", { username, password });
        toast.success("Account created — please log in");
        setMode("login");
      } else {
        const resp = await api.post<{ token: string }>("/auth/login", { username, password });
        setToken(resp.token);
        navigate({ to: "/" });
      }
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex min-h-[60vh] items-center justify-center">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>{mode === "login" ? "Sign In" : "Create Account"}</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1">
              <Label htmlFor="username">Username</Label>
              <Input
                id="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? "..." : mode === "login" ? "Sign In" : "Register"}
            </Button>
            <p className="text-center text-sm text-muted-foreground">
              {mode === "login" ? "No account? " : "Have an account? "}
              <button
                type="button"
                className="underline"
                onClick={() => setMode(mode === "login" ? "register" : "login")}
              >
                {mode === "login" ? "Register" : "Sign In"}
              </button>
            </p>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
