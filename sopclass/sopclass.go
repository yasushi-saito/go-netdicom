package sopclass

// TODO(saito) Merge w/ dicomuid.

// DICOM SOP UID listing.
//
// https://www.dicomlibrary.com/dicom/sop/
//
// Translate from sop_class.py in pynetdicom3; https://github.com/pydicom/pynetdicom3
type SOPUID struct {
	Name string
	UID  string
}

// For issuing C-ECHO
var VerificationClasses = []SOPUID{
	SOPUID{"VerificationSOPClass", "1.2.840.10008.1.1"},
}

// For issuing C-STORE or C-GET
var StorageClasses = []SOPUID{
	SOPUID{"ComputedRadiographyImageStorage", "1.2.840.10008.5.1.4.1.1.1"},
	SOPUID{"DigitalXRayImagePresentationStorage", "1.2.840.10008.5.1.4.1.1.1.1"},
	SOPUID{"DigitalMammographyXRayImagePresentationStorage", "1.2.840.10008.5.1.4.1.1.1.2"},
	SOPUID{"DigitalMammographyXRayImageProcessingStorage", "1.2.840.10008.5.1.4.1.1.1.2.1"},
	SOPUID{"DigitalIntraOralXRayImagePresentationStorage", "1.2.840.10008.5.1.4.1.1.1.3"},
	SOPUID{"CTImageStorage", "1.2.840.10008.5.1.4.1.1.2"},
	SOPUID{"EnhancedCTImageStorage", "1.2.840.10008.5.1.4.1.1.2.1"},
	SOPUID{"LegacyConvertedEnhancedCTImageStorage", "1.2.840.10008.5.1.4.1.1.2.2"},
	SOPUID{"UltrasoundMultiframeImageStorage", "1.2.840.10008.5.1.4.1.1.3.1"},
	SOPUID{"MRImageStorage", "1.2.840.10008.5.1.4.1.1.4"},
	SOPUID{"EnhancedMRImageStorage", "1.2.840.10008.5.1.4.1.1.4.1"},
	SOPUID{"MRSpectroscopyStorage", "1.2.840.10008.5.1.4.1.1.4.2"},
	SOPUID{"EnhancedMRColorImageStorage", "1.2.840.10008.5.1.4.1.1.4.3"},
	SOPUID{"LegacyConvertedEnhancedMRImageStorage", "1.2.840.10008.5.1.4.1.1.4.4"},
	SOPUID{"UltrasoundImageStorage", "1.2.840.10008.5.1.4.1.1.6.1"},
	SOPUID{"EnhancedUSVolumeStorage", "1.2.840.10008.5.1.4.1.1.6.2"},
	SOPUID{"SecondaryCaptureImageStorage", "1.2.840.10008.5.1.4.1.1.7"},
	SOPUID{"MultiframeSingleBitSecondaryCaptureImageStorage", "1.2.840.10008.5.1.4.1.1.7.1"},
	SOPUID{"MultiframeGrayscaleByteSecondaryCaptureImageStorage", "1.2.840.10008.5.1.4.1.1.7.2"},
	SOPUID{"MultiframeGrayscaleWordSecondaryCaptureImageStorage", "1.2.840.10008.5.1.4.1.1.7.3"},
	SOPUID{"MultiframeTrueColorSecondaryCaptureImageStorage", "1.2.840.10008.5.1.4.1.1.7.4"},
	SOPUID{"TwelveLeadECGWaveformStorage", "1.2.840.10008.5.1.4.1.1.9.1.1"},
	SOPUID{"GeneralECGWaveformStorage", "1.2.840.10008.5.1.4.1.1.9.1.2"},
	SOPUID{"AmbulatoryECGWaveformStorage", "1.2.840.10008.5.1.4.1.1.9.1.3"},
	SOPUID{"HemodynamicWaveformStorage", "1.2.840.10008.5.1.4.1.1.9.2.1"},
	SOPUID{"CardiacElectrophysiologyWaveformStorage", "1.2.840.10008.5.1.4.1.1.9.3.1"},
	SOPUID{"BasicVoiceAudioWaveformStorage", "1.2.840.10008.5.1.4.1.1.9.4.1"},
	SOPUID{"GeneralAudioWaveformStorage", "1.2.840.10008.5.1.4.1.1.9.4.2"},
	SOPUID{"ArterialPulseWaveformStorage", "1.2.840.10008.5.1.4.1.1.9.5.1"},
	SOPUID{"RespiratoryWaveformStorage", "1.2.840.10008.5.1.4.1.1.9.6.1"},
	SOPUID{"GrayscaleSoftcopyPresentationStateStorage", "1.2.840.10008.5.1.4.1.1.11.1"},
	SOPUID{"ColorSoftcopyPresentationStateStorage", "1.2.840.10008.5.1.4.1.1.11.2"},
	SOPUID{"PseudocolorSoftcopyPresentationStageStorage", "1.2.840.10008.5.1.4.1.1.11.3"},
	SOPUID{"BlendingSoftcopyPresentationStateStorage", "1.2.840.10008.5.1.4.1.1.11.4"},
	SOPUID{"XAXRFGrayscaleSoftcopyPresentationStateStorage", "1.2.840.10008.5.1.4.1.1.11.5"},
	SOPUID{"XRayAngiographicImageStorage", "1.2.840.10008.5.1.4.1.1.12.1"},
	SOPUID{"EnhancedXAImageStorage", "1.2.840.10008.5.1.4.1.1.12.1.1"},
	SOPUID{"XRayRadiofluoroscopicImageStorage", "1.2.840.10008.5.1.4.1.1.12.2"},
	SOPUID{"EnhancedXRFImageStorage", "1.2.840.10008.5.1.4.1.1.12.2.1"},
	SOPUID{"XRay3DAngiographicImageStorage", "1.2.840.10008.5.1.4.1.1.13.1.1"},
	SOPUID{"XRay3DCraniofacialImageStorage", "1.2.840.10008.5.1.4.1.1.13.1.2"},
	SOPUID{"BreastTomosynthesisImageStorage", "1.2.840.10008.5.1.4.1.1.13.1.3"},
	SOPUID{"BreastProjectionXRayImagePresentationStorage", "1.2.840.10008.5.1.4.1.1.13.1.4"},
	SOPUID{"BreastProjectionXRayImageProcessingStorage", "1.2.840.10008.5.1.4.1.1.13.1.5"},
	SOPUID{"IntravascularOpticalCoherenceTomographyImagePresentationStorage", "1.2.840.10008.5.1.4.1.1.14.1"},
	SOPUID{"IntravascularOpticalCoherenceTomographyImageProcessingStorage", "1.2.840.10008.5.1.4.1.1.14.2"},
	SOPUID{"NuclearMedicineImageStorage", "1.2.840.10008.5.1.4.1.1.20"},
	SOPUID{"ParametricMapStorage", "1.2.840.10008.5.1.4.1.1.30"},
	SOPUID{"RawDataStorage", "1.2.840.10008.5.1.4.1.1.66"},
	SOPUID{"SpatialRegistrationStorage", "1.2.840.10008.5.1.4.1.1.66.1"},
	SOPUID{"SpatialFiducialsStorage", "1.2.840.10008.5.1.4.1.1.66.2"},
	SOPUID{"DeformableSpatialRegistrationStorage", "1.2.840.10008.5.1.4.1.1.66.3"},
	SOPUID{"SegmentationStorage", "1.2.840.10008.5.1.4.1.1.66.4"},
	SOPUID{"SurfaceSegmentationStorage", "1.2.840.10008.5.1.4.1.1.66.5"},
	SOPUID{"RealWorldValueMappingStorage", "1.2.840.10008.5.1.4.1.1.67"},
	SOPUID{"SurfaceScanMeshStorage", "1.2.840.10008.5.1.4.1.1.68.1"},
	SOPUID{"SurfaceScanPointCloudStorage", "1.2.840.10008.5.1.4.1.1.68.2"},
	SOPUID{"VLEndoscopicImageStorage", "1.2.840.10008.5.1.4.1.1.77.1.1"},
	SOPUID{"VideoEndoscopicImageStorage", "1.2.840.10008.5.1.4.1.1.77.1.1.1"},
	SOPUID{"VLMicroscopicImageStorage", "1.2.840.10008.5.1.4.1.1.77.1.2"},
	SOPUID{"VideoMicroscopicImageStorage", "1.2.840.10008.5.1.4.1.1.77.1.2.1"},
	SOPUID{"VLSlideCoordinatesMicroscopicImageStorage", "1.2.840.10008.5.1.4.1.1.77.1.3"},
	SOPUID{"VLPhotographicImageStorage", "1.2.840.10008.5.1.4.1.1.77.1.4"},
	SOPUID{"VideoPhotographicImageStorage", "1.2.840.10008.5.1.4.1.1.77.1.4.1"},
	SOPUID{"OphthalmicPhotography8BitImageStorage", "1.2.840.10008.5.1.4.1.1.77.1.5.1"},
	SOPUID{"OphthalmicPhotography16BitImageStorage", "1.2.840.10008.5.1.4.1.1.77.1.5.2"},
	SOPUID{"StereometricRelationshipStorage", "1.2.840.10008.5.1.4.1.1.77.1.5.3"},
	SOPUID{"OpthalmicTomographyImageStorage", "1.2.840.10008.5.1.4.1.1.77.1.5.4"},
	SOPUID{"WideFieldOpthalmicPhotographyStereographicProjectionImageStorage", "1.2.840.10008.5.1.4.1.1.77.1.5.5"},
	SOPUID{"WideFieldOpthalmicPhotography3DCoordinatesImageStorage", "1.2.840.10008.5.1.4.1.1.77.1.5.6"},
	SOPUID{"VLWholeSlideMicroscopyImageStorage", "1.2.840.10008.5.1.4.1.1.77.1.6"},
	SOPUID{"LensometryMeasurementsStorage", "1.2.840.10008.5.1.4.1.1.78.1"},
	SOPUID{"AutorefractionMeasurementsStorage", "1.2.840.10008.5.1.4.1.1.78.2"},
	SOPUID{"KeratometryMeasurementsStorage", "1.2.840.10008.5.1.4.1.1.78.3"},
	SOPUID{"SubjectiveRefractionMeasurementsStorage", "1.2.840.10008.5.1.4.1.1.78.4"},
	SOPUID{"VisualAcuityMeasurementsStorage", "1.2.840.10008.5.1.4.1.1.78.5"},
	SOPUID{"SpectaclePrescriptionReportStorage", "1.2.840.10008.5.1.4.1.1.78.6"},
	SOPUID{"OpthalmicAxialMeasurementsStorage", "1.2.840.10008.5.1.4.1.1.78.7"},
	SOPUID{"IntraocularLensCalculationsStorage", "1.2.840.10008.5.1.4.1.1.78.8"},
	SOPUID{"MacularGridThicknessAndVolumeReport", "1.2.840.10008.5.1.4.1.1.79.1"},
	SOPUID{"OpthalmicVisualFieldStaticPerimetryMeasurementsStorag", "1.2.840.10008.5.1.4.1.1.80.1"},
	SOPUID{"OpthalmicThicknessMapStorage", "1.2.840.10008.5.1.4.1.1.81.1"},
	SOPUID{"CornealTopographyMapStorage", "1.2.840.10008.5.1.4.1.1.82.1"},
	SOPUID{"BasicTextSRStorage", "1.2.840.10008.5.1.4.1.1.88.11"},
	SOPUID{"EnhancedSRStorage", "1.2.840.10008.5.1.4.1.1.88.22"},
	SOPUID{"ComprehensiveSRStorage", "1.2.840.10008.5.1.4.1.1.88.33"},
	SOPUID{"Comprehenseice3DSRStorage", "1.2.840.10008.5.1.4.1.1.88.34"},
	SOPUID{"ExtensibleSRStorage", "1.2.840.10008.5.1.4.1.1.88.35"},
	SOPUID{"ProcedureSRStorage", "1.2.840.10008.5.1.4.1.1.88.40"},
	SOPUID{"MammographyCADSRStorage", "1.2.840.10008.5.1.4.1.1.88.50"},
	SOPUID{"KeyObjectSelectionStorage", "1.2.840.10008.5.1.4.1.1.88.59"},
	SOPUID{"ChestCADSRStorage", "1.2.840.10008.5.1.4.1.1.88.65"},
	SOPUID{"XRayRadiationDoseSRStorage", "1.2.840.10008.5.1.4.1.1.88.67"},
	SOPUID{"RadiopharmaceuticalRadiationDoseSRStorage", "1.2.840.10008.5.1.4.1.1.88.68"},
	SOPUID{"ColonCADSRStorage", "1.2.840.10008.5.1.4.1.1.88.69"},
	SOPUID{"ImplantationPlanSRDocumentStorage", "1.2.840.10008.5.1.4.1.1.88.70"},
	SOPUID{"EncapsulatedPDFStorage", "1.2.840.10008.5.1.4.1.1.104.1"},
	SOPUID{"EncapsulatedCDAStorage", "1.2.840.10008.5.1.4.1.1.104.2"},
	SOPUID{"PositronEmissionTomographyImageStorage", "1.2.840.10008.5.1.4.1.1.128"},
	SOPUID{"EnhancedPETImageStorage", "1.2.840.10008.5.1.4.1.1.130"},
	SOPUID{"LegacyConvertedEnhancedPETImageStorage", "1.2.840.10008.5.1.4.1.1.128.1"},
	SOPUID{"BasicStructuredDisplayStorage", "1.2.840.10008.5.1.4.1.1.131"},
	SOPUID{"RTImageStorage", "1.2.840.10008.5.1.4.1.1.481.1"},
	SOPUID{"RTDoseStorage", "1.2.840.10008.5.1.4.1.1.481.2"},
	SOPUID{"RTStructureSetStorage", "1.2.840.10008.5.1.4.1.1.481.3"},
	SOPUID{"RTBeamsTreatmentRecordStorage", "1.2.840.10008.5.1.4.1.1.481.4"},
	SOPUID{"RTPlanStorage", "1.2.840.10008.5.1.4.1.1.481.5"},
	SOPUID{"RTBrachyTreatmentRecordStorage", "1.2.840.10008.5.1.4.1.1.481.6"},
	SOPUID{"RTTreatmentSummaryRecordStorage", "1.2.840.10008.5.1.4.1.1.481.7"},
	SOPUID{"RTIonPlanStorage", "1.2.840.10008.5.1.4.1.1.481.8"},
	SOPUID{"RTIonBeamsTreatmentRecordStorage", "1.2.840.10008.5.1.4.1.1.481.9"},
	SOPUID{"RTBeamsDeliveryInstructionStorage", "1.2.840.10008.5.1.4.34.7"},
	SOPUID{"GenericImplantTemplateStorage", "1.2.840.10008.5.1.4.43.1"},
	SOPUID{"ImplantAssemblyTemplateStorage", "1.2.840.10008.5.1.4.44.1"},
	SOPUID{"ImplantTemplateGroupStorage", "1.2.840.10008.5.1.4.45.1"},
}

// For issuing C-FIND
var QRFindClasses = []SOPUID{
	SOPUID{"PatientRootQueryRetrieveInformationModelFind", "1.2.840.10008.5.1.4.1.2.1.1"},
	SOPUID{"StudyRootQueryRetrieveInformationModelFind", "1.2.840.10008.5.1.4.1.2.2.1"},
	SOPUID{"PatientStudyOnlyQueryRetrieveInformationModelFind", "1.2.840.10008.5.1.4.1.2.3.1"},
	SOPUID{"ModalityWorklistInformationFind", "1.2.840.10008.5.1.4.31"}}

// For issuing C-MOVE
var QRMoveClasses = []SOPUID{
	SOPUID{"PatientRootQueryRetrieveInformationModelMove", "1.2.840.10008.5.1.4.1.2.1.2"},
	SOPUID{"StudyRootQueryRetrieveInformationModelMove", "1.2.840.10008.5.1.4.1.2.2.2"},
	SOPUID{"PatientStudyOnlyQueryRetrieveInformationModelMove", "1.2.840.10008.5.1.4.1.2.3.2"}}

// TODO(saito) Does this really work?
var QRGetClasses = []SOPUID{
	SOPUID{"PatientRootQueryRetrieveInformationModelGet", "1.2.840.10008.5.1.4.1.2.1.3"},
	SOPUID{"StudyRootQueryRetrieveInformationModelGet", "1.2.840.10008.5.1.4.1.2.2.3"},
	SOPUID{"PatientStudyOnlyQueryRetrieveInformationModelGet", "1.2.840.10008.5.1.4.1.2.3.3"}}
