"use client";

import { cn } from "cn";
import { FileTextIcon, UploadIcon } from "lucide-react";
import { useFormatter, useTranslations } from "next-intl";
import { useRef, useState } from "react";

export function CSVFileField({ describedBy, disabled, invalid }: {
  describedBy?: string;
  disabled: boolean;
  invalid?: boolean;
}) {
  const t = useTranslations("Imports");
  const format = useFormatter();
  const input = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [dragging, setDragging] = useState(false);
  const [wrongFile, setWrongFile] = useState(false);
  const Icon = file ? FileTextIcon : UploadIcon;

  function selectFiles(files: FileList | null) {
    const field = input.current;
    if (!field) return;
    const selected = files?.[0];
    const valid = files?.length === 1 && selected && (/\.csv$/i.test(selected.name) || selected.type === "text/csv");
    setWrongFile(Boolean(files?.length && !valid));
    if (valid) {
      field.files = files;
    } else {
      field.value = "";
    }
    setFile(valid ? selected : null);
  }

  return (
    <div>
      <div
        className={cn(
          "relative flex min-h-28 w-full items-center gap-3 rounded-lg border border-dashed border-input bg-card px-4 py-4",
          "focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-ring",
          disabled ? "border-border bg-background" : "hover:bg-accent",
          dragging && "border-ring bg-accent",
          (invalid || wrongFile) && "border-destructive",
        )}
        onDragOver={(event) => {
          if (!event.dataTransfer.types.includes("Files")) return;
          event.preventDefault();
          event.dataTransfer.dropEffect = disabled ? "none" : "copy";
          if (!disabled) setDragging(true);
        }}
        onDragLeave={(event) => {
          if (!(event.relatedTarget instanceof Node) || !event.currentTarget.contains(event.relatedTarget)) setDragging(false);
        }}
        onDrop={(event) => {
          event.preventDefault();
          setDragging(false);
          if (!disabled && event.dataTransfer.files.length) selectFiles(event.dataTransfer.files);
        }}
      >
        <input
          accept=".csv,text/csv"
          aria-describedby={["csv-file-hint", describedBy, wrongFile && "csv-file-error"].filter(Boolean).join(" ")}
          aria-invalid={invalid || wrongFile}
          className="absolute inset-0 size-full cursor-pointer opacity-0 disabled:cursor-not-allowed"
          disabled={disabled}
          id="csv-file"
          name="file"
          onChange={(event) => selectFiles(event.currentTarget.files)}
          ref={input}
          required
          type="file"
        />
        <Icon aria-hidden="true" className="size-5 shrink-0 text-primary" />
        <div className="min-w-0 text-sm">
          <label className="font-medium" htmlFor="csv-file">{t("fields.file")}</label>
          <p id="csv-file-hint" className="mt-1 text-muted-foreground">{t("uploadHint")}</p>
          <div aria-live="polite">
            {file && (
              <p className="mt-2 wrap-anywhere">
                <span className="font-medium">{file.name}</span>
                {" · "}
                <span className="text-muted-foreground">
                  {file.size < 1000
                    ? format.number(file.size, { style: "unit", unit: "byte", unitDisplay: "long" })
                    : format.number(file.size / 1000, { style: "unit", unit: "kilobyte", maximumFractionDigits: 1 })}
                </span>
              </p>
            )}
          </div>
        </div>
      </div>
      {wrongFile && <p className="mt-2 text-sm text-destructive" id="csv-file-error" role="alert">{t("uploadOneCSV")}</p>}
    </div>
  );
}
