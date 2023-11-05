import { isApiValidationError, mapApiErrorToFieldErrors } from "api/errors";
import { FormikContextType, FormikErrors, getIn } from "formik";
import {
  ChangeEvent,
  ChangeEventHandler,
  FocusEventHandler,
  ReactNode,
} from "react";
import * as Yup from "yup";

export const validationText = {
  nameRequired: (name: string): string => {
    return name ? `Please enter a ${name.toLowerCase()}.` : "Required";
  },
  nameInvalidChars: (name: string): string => {
    return `${name} must start with a-Z or 0-9 and can contain a-Z, 0-9 or -`;
  },
  nameTooLong: (name: string, len: number): string => {
    return `${name} cannot be longer than ${len} characters`;
  },
  templateDisplayNameInvalidChars: (name: string): string => {
    return `${name} must start and end with non-whitespace character`;
  },
  workspaceNameInvalidChars: (name: string): string => {
    return `${name} must start with lowercase a-z or 0-9 and can contain lowercase a-z, 0-9 or -`;
  },
};

interface FormHelpers {
  name: string;
  onBlur: FocusEventHandler;
  onChange: ChangeEventHandler;
  id: string;
  value?: string | number;
  error: boolean;
  helperText?: ReactNode;
}

export const getFormHelpers =
  <TFormValues>(form: FormikContextType<TFormValues>, error?: unknown) =>
  (
    fieldName: keyof TFormValues | string,
    helperText?: ReactNode,
    // backendFieldName is used when the value in the form is named different from the backend
    backendFieldName?: string,
  ): FormHelpers => {
    const apiValidationErrors = isApiValidationError(error)
      ? (mapApiErrorToFieldErrors(
          error.response.data,
        ) as FormikErrors<TFormValues> & { [key: string]: string })
      : undefined;
    // Since the fieldName can be a path string like parameters[0].value we need to use getIn
    const touched = Boolean(getIn(form.touched, fieldName.toString()));
    const formError = getIn(form.errors, fieldName.toString());
    // Since the field in the form can be diff from the backend, we need to
    // check for both when getting the error
    const apiField = backendFieldName ?? fieldName;
    const apiError = apiValidationErrors?.[apiField.toString()];
    const errorToDisplay = apiError ?? formError;

    return {
      ...form.getFieldProps(fieldName),
      id: fieldName.toString(),
      error: touched && Boolean(errorToDisplay),
      helperText: touched ? errorToDisplay ?? helperText : helperText,
    };
  };

export const onChangeTrimmed =
  <T>(form: FormikContextType<T>, callback?: (value: string) => void) =>
  (event: ChangeEvent<HTMLInputElement>): void => {
    event.target.value = event.target.value.trim();
    form.handleChange(event);
    callback?.(event.target.value);
  };

// REMARK: Keep these consts in sync with coderd/httpapi/httpapi.go
const maxLenName = 32;
const templateDisplayNameMaxLength = 64;
const usernameRE = /^[a-zA-Z0-9]+(?:-[a-zA-Z0-9]+)*$/;
const templateDisplayNameRE = /^[^\s](.*[^\s])?$/;
const workspaceNameRE = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

// REMARK: see #1756 for name/username semantics
export const nameValidator = (name: string): Yup.StringSchema =>
  Yup.string()
    .required(validationText.nameRequired(name))
    .matches(usernameRE, validationText.nameInvalidChars(name))
    .max(maxLenName, validationText.nameTooLong(name, maxLenName));

export const templateDisplayNameValidator = (
  displayName: string,
): Yup.StringSchema =>
  Yup.string()
    .matches(
      templateDisplayNameRE,
      validationText.templateDisplayNameInvalidChars(displayName),
    )
    .max(
      templateDisplayNameMaxLength,
      validationText.nameTooLong(displayName, templateDisplayNameMaxLength),
    )
    .optional();

export const workspaceNameValidator = (name: string): Yup.StringSchema =>
  Yup.string()
    .required(validationText.nameRequired(name))
    .matches(workspaceNameRE, validationText.workspaceNameInvalidChars(name));

export const iconValidator = Yup.string().label("Icon").max(256);
