import { Link, useNavigate } from "@tanstack/react-router";
import { Button } from "@frontend/ui/components/button";

import { ModeToggle } from "./mode-toggle";
import { clearToken, isAuthenticated } from "@/lib/auth";

export default function Header() {
  const navigate = useNavigate();
  const authed = isAuthenticated();

  function logout() {
    clearToken();
    navigate({ to: "/login" });
  }

  const links = authed
    ? [
        { to: "/", label: "Dashboard" },
        { to: "/subscriptions", label: "Subscriptions" },
        { to: "/scheduler", label: "Scheduler" },
        { to: "/settings/notify", label: "Notify" },
        { to: "/settings/general", label: "Settings" },
      ]
    : [];

  return (
    <div>
      <div className="flex flex-row items-center justify-between px-4 py-2">
        <nav className="flex gap-4">
          {links.map(({ to, label }) => (
            <Link key={to} to={to} className="text-sm font-medium hover:underline">
              {label}
            </Link>
          ))}
        </nav>
        <div className="flex items-center gap-2">
          <ModeToggle />
          {authed && (
            <Button variant="outline" size="sm" onClick={logout}>
              Logout
            </Button>
          )}
        </div>
      </div>
      <hr />
    </div>
  );
}
