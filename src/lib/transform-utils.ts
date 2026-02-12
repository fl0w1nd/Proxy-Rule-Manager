import { Transform } from "./schema";

export type TransformType = Transform["type"];

export function createTransformByType(type: TransformType): Transform {
  if (type === "replace") {
    return {
      type,
      target: "all",
      pattern: "",
      replacement: "",
    };
  }

  if (type === "remove_lines") {
    return {
      type,
      target: "all",
      pattern: "",
    };
  }

  return {
    type,
    target: "all",
  };
}

export function getTransformTypeUpdates(type: TransformType): Partial<Transform> {
  if (type === "replace") {
    return {
      type,
      use: undefined,
      pattern: "",
      replacement: "",
      flags: undefined,
    };
  }

  if (type === "remove_lines") {
    return {
      type,
      use: undefined,
      pattern: "",
      replacement: undefined,
      flags: undefined,
    };
  }

  return {
    type,
    use: "",
    pattern: undefined,
    replacement: undefined,
    flags: undefined,
  };
}
