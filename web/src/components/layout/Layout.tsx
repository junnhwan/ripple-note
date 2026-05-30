import { Outlet } from "react-router-dom";
import Navbar from "./Navbar";
import MobileNav from "./MobileNav";
import { Toaster } from "sonner";

export default function Layout() {
  return (
    <div className="min-h-screen bg-background">
      <Navbar />
      <main className="mx-auto max-w-6xl px-4 pb-20 pt-4 md:pb-6">
        <Outlet />
      </main>
      <MobileNav />
      <Toaster position="top-center" richColors closeButton />
    </div>
  );
}
