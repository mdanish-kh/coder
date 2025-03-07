import { type FC } from "react";
import { Helmet } from "react-helmet-async";
import { useNavigate, useSearchParams } from "react-router-dom";
import { pageTitle } from "utils/page";
import { FullPageHorizontalForm } from "components/FullPageForm/FullPageHorizontalForm";
import { DuplicateTemplateView } from "./DuplicateTemplateView";
import { ImportStarterTemplateView } from "./ImportStarterTemplateView";
import { UploadTemplateView } from "./UploadTemplateView";
import { Template } from "api/typesGenerated";

const CreateTemplatePage: FC = () => {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  const onSuccess = (template: Template) => {
    navigate(`/templates/${template.name}/files`);
  };

  const onCancel = () => {
    navigate(-1);
  };

  return (
    <>
      <Helmet>
        <title>{pageTitle("Create Template")}</title>
      </Helmet>

      <FullPageHorizontalForm title="Create Template" onCancel={onCancel}>
        {searchParams.has("fromTemplate") ? (
          <DuplicateTemplateView onSuccess={onSuccess} />
        ) : searchParams.has("exampleId") ? (
          <ImportStarterTemplateView onSuccess={onSuccess} />
        ) : (
          <UploadTemplateView onSuccess={onSuccess} />
        )}
      </FullPageHorizontalForm>
    </>
  );
};

export default CreateTemplatePage;
