"use client";

import { cn } from "cn";
import { CheckIcon, ChevronDownIcon } from "lucide-react";
import { Select } from "radix-ui";
import type { ComponentProps, ReactNode } from "react";

type FormSelectProps = Pick<ComponentProps<typeof Select.Root>,
  "defaultValue" | "disabled" | "name" | "onValueChange" | "required" | "value"> &
  Pick<ComponentProps<typeof Select.Trigger>, "aria-describedby" | "aria-invalid" | "className" | "id"> & {
    options: readonly { value: string; label: ReactNode; disabled?: boolean }[];
    placeholder?: string;
  };

export function FormSelect({
  options, placeholder, className, id, "aria-describedby": describedBy, "aria-invalid": invalid, ...props
}: FormSelectProps) {
  return (
    <Select.Root {...props}>
      <Select.Trigger
        aria-describedby={describedBy}
        aria-invalid={invalid}
        className={cn("form-control flex h-11 w-full items-center justify-between gap-2 px-3 text-left text-sm data-placeholder:text-muted-foreground [&>span:first-child]:truncate", className)}
        id={id}
      >
        <Select.Value placeholder={placeholder} />
        <Select.Icon asChild>
          <ChevronDownIcon aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
        </Select.Icon>
      </Select.Trigger>
      <Select.Portal>
        <Select.Content
          align="start"
          className="choice-menu max-h-(--radix-select-content-available-height) min-w-(--radix-select-trigger-width) max-w-(--radix-select-content-available-width)"
          collisionPadding={16}
          position="popper"
          sideOffset={6}
        >
          <Select.Viewport>
            {options.map(({ value, label, disabled }) => (
              <Select.Item className="choice-option" disabled={disabled} key={value} value={value}>
                <Select.ItemText>{label}</Select.ItemText>
                <Select.ItemIndicator>
                  <CheckIcon aria-hidden="true" className="size-4 shrink-0 text-primary" />
                </Select.ItemIndicator>
              </Select.Item>
            ))}
          </Select.Viewport>
        </Select.Content>
      </Select.Portal>
    </Select.Root>
  );
}
