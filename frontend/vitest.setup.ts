import "@testing-library/jest-dom/vitest";

// jsdom's Blob does not implement arrayBuffer(), which the streaming hasher uses. Real browsers
// all support it; polyfill it here via FileReader so hashing tests run under jsdom.
if (typeof Blob !== "undefined" && !Blob.prototype.arrayBuffer) {
  Blob.prototype.arrayBuffer = function arrayBuffer(): Promise<ArrayBuffer> {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result as ArrayBuffer);
      reader.onerror = () => reject(reader.error);
      reader.readAsArrayBuffer(this);
    });
  };
}
