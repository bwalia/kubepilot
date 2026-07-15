import { Html, Head, Main, NextScript } from "next/document";

/**
 * Custom document. The inline script applies the operator's saved theme
 * ("kubepilot-theme" in localStorage) to <html data-theme> BEFORE first paint,
 * so switching to Night mode never flashes the default Daylight deck on reload.
 * Default (no saved preference) is Daylight, which is what :root already renders.
 */
const NO_FLASH = `
(function(){try{var t=localStorage.getItem('kubepilot-theme');if(t==='dark'||t==='light'){document.documentElement.setAttribute('data-theme',t);}}catch(e){}})();
`;

export default function Document() {
  return (
    <Html lang="en">
      <Head>
        <meta name="color-scheme" content="light dark" />
        <script dangerouslySetInnerHTML={{ __html: NO_FLASH }} />
      </Head>
      <body>
        <Main />
        <NextScript />
      </body>
    </Html>
  );
}
