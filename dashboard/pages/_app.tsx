import type { AppProps } from "next/app";
import { useRouter } from "next/router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { GlobalNav } from "@/components/GlobalNav";
import { Footer } from "@/components/Footer";
import "../styles/globals.css";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Refresh cluster data every 15 seconds to keep the cockpit live.
      refetchInterval: 15_000,
      staleTime: 10_000,
    },
  },
});

export default function App({ Component, pageProps }: AppProps) {
  const router = useRouter();
  const reduce = useReducedMotion();

  return (
    <QueryClientProvider client={queryClient}>
      <div className="flex min-h-screen flex-col">
        <GlobalNav />
        <div className="flex-1">
          {/* Subtle cross-fade + rise between routes; exit is faster than enter
              so back/forward feels snappy. Silenced under reduced-motion. */}
          <AnimatePresence mode="wait" initial={false}>
            <motion.div
              key={router.asPath}
              initial={reduce ? false : { opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={reduce ? undefined : { opacity: 0, y: -6 }}
              transition={{ duration: reduce ? 0 : 0.28, ease: [0.22, 1, 0.36, 1] }}
            >
              <Component {...pageProps} />
            </motion.div>
          </AnimatePresence>
        </div>
        <Footer />
      </div>
    </QueryClientProvider>
  );
}
