import { useEffect, useRef, useState } from "react";

type UseClipboardResult = Readonly<{
  isCopied: boolean;
  copyToClipboard: () => Promise<void>;
}>;

export const useClipboard = (textToCopy: string): UseClipboardResult => {
  const [isCopied, setIsCopied] = useState(false);
  const timeoutIdRef = useRef<number | undefined>();

  useEffect(() => {
    const clearIdsOnUnmount = () => window.clearTimeout(timeoutIdRef.current);
    return clearIdsOnUnmount;
  }, []);

  const copyToClipboard = async () => {
    try {
      await window.navigator.clipboard.writeText(textToCopy);
      setIsCopied(true);
      timeoutIdRef.current = window.setTimeout(() => {
        setIsCopied(false);
      }, 1000);
    } catch (err) {
      const isCopied = simulateClipboardWrite();
      if (isCopied) {
        setIsCopied(true);
        timeoutIdRef.current = window.setTimeout(() => {
          setIsCopied(false);
        }, 1000);
      } else {
        const wrappedErr = new Error(
          "copyToClipboard: failed to copy text to clipboard",
        );
        if (err instanceof Error) {
          wrappedErr.stack = err.stack;
        }
        console.error(wrappedErr);
      }
    }
  };

  return { isCopied, copyToClipboard };
};

/**
 * It feels silly that you have to make a whole dummy input just to simulate a
 * clipboard, but that's really the recommended approach for older browsers.
 *
 * @see {@link https://web.dev/patterns/clipboard/copy-text?hl=en}
 */
function simulateClipboardWrite(): boolean {
  const previousFocusTarget = document.activeElement;
  const dummyInput = document.createElement("input");

  // Using visually-hidden styling to ensure that inserting the element doesn't
  // cause any content reflows on the page (removes any risk of UI flickers).
  // Can't use visibility:hidden or display:none, because then the elements
  // can't receive focus, which is needed for the execCommand method to work
  const style = dummyInput.style;
  style.display = "inline-block";
  style.position = "absolute";
  style.overflow = "hidden";
  style.clip = "rect(0 0 0 0)";
  style.clipPath = "rect(0 0 0 0)";
  style.height = "1px";
  style.width = "1px";
  style.margin = "-1px";
  style.padding = "0";
  style.border = "0";

  document.body.appendChild(dummyInput);
  dummyInput.focus();
  dummyInput.select();

  const isCopied = document.execCommand("copy");
  dummyInput.remove();

  if (previousFocusTarget instanceof HTMLElement) {
    previousFocusTarget.focus();
  }

  return isCopied;
}
