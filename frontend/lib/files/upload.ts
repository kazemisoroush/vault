// uploadBytes PUTs the raw file to a presigned S3 URL. The bytes never pass through the API.
export async function uploadBytes(url: string, file: File, contentType: string): Promise<void> {
  const response = await fetch(url, {
    method: "PUT",
    headers: { "Content-Type": contentType },
    body: file,
  });
  if (!response.ok) {
    throw new Error(`upload failed: ${response.status}`);
  }
}
