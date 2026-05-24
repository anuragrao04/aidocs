import React from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "sonner";
import { TooltipProvider } from "@radix-ui/react-tooltip";
import "./styles.css";
import { ThemeProvider } from "@/lib/theme";
import { AppShell } from "@/components/layout/app-shell";
import { LandingPage } from "@/pages/landing";
import { LoginPage } from "@/pages/login";
import { DocumentsPage } from "@/pages/documents";
import { DocumentPage } from "@/pages/document";
import { ServiceAccountsPage } from "@/pages/service-accounts";
import { DevelopersPage } from "@/pages/developers";
import { ProfilePage } from "@/pages/profile";
import { StartPage } from "@/pages/start";

const qc = new QueryClient({
  defaultOptions: { queries: { refetchOnWindowFocus: false } },
});

function App() {
  return (
    <ThemeProvider>
      <QueryClientProvider client={qc}>
        <TooltipProvider delayDuration={150}>
          <BrowserRouter>
            <Routes>
              <Route path="/" element={<LandingPage />} />
              <Route path="/login" element={<LoginPage />} />
              <Route path="/app" element={<AppShell />}>
                <Route index element={<Navigate to="documents" replace />} />
                <Route path="start" element={<StartPage />} />
                <Route path="documents" element={<DocumentsPage />} />
                <Route path="d/:id" element={<DocumentPage />} />
                <Route
                  path="settings/service-accounts"
                  element={<ServiceAccountsPage />}
                />
                <Route path="settings/profile" element={<ProfilePage />} />
                <Route path="developers" element={<DevelopersPage />} />
              </Route>
              {/* legacy redirects */}
              <Route
                path="/documents/:id"
                element={<LegacyDocRedirect />}
              />
              <Route
                path="/service-accounts"
                element={
                  <Navigate to="/app/settings/service-accounts" replace />
                }
              />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
            <Toaster position="bottom-right" theme="system" richColors />
          </BrowserRouter>
        </TooltipProvider>
      </QueryClientProvider>
    </ThemeProvider>
  );
}

function LegacyDocRedirect() {
  const id = window.location.pathname.split("/").pop() || "";
  return <Navigate to={`/app/d/${id}`} replace />;
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
